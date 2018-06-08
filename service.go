package main

import (
  "fmt"
  "github.com/go-redis/redis"
  "log"
  "os"
  "os/exec"
  "strings"
  "sync"
  "syscall"
)

var (
  RedisHost = os.Getenv("REDIS_HOST")
  BroadcastIP = os.Getenv("BROADCAST_IP")
  FirehousePath = os.Getenv("firehouse_path")
  LogPath = os.Getenv("logs_path")
  ServiceLog *log.Logger
  ServiceLogPath = LogPath + "/audioplayer.log"
  ErrorLog *log.Logger
  ErrorLogPath = LogPath + "/audioplayer.error"
  ClientePlay = clienteRedis()
  ClienteStop = clienteRedis()
  PubSubPlay *redis.PubSub
  PubSubStop *redis.PubSub
  VLCpid int
)

func init() {
  // Configuración de log de servicio
  fileHandlerS, errS := os.OpenFile(ServiceLogPath, os.O_RDWR | os.O_CREATE | os.O_APPEND, 0664)
  if errS != nil {
    log.Fatal("Error al abrir el Log de Servicio - " + errS.Error())
  }

  ServiceLog = log.New(fileHandlerS, "[INFO] ", log.Ldate|log.Ltime)

  // Configuracion de log de error
  fileHandlerE, errE := os.OpenFile(ErrorLogPath, os.O_RDWR | os.O_CREATE | os.O_APPEND, 0664)
  if errE != nil {
    log.Fatal("Error al abrir el Log de Error - " + errE.Error())
  }

  ErrorLog = log.New(fileHandlerE, "[ERROR] ", log.Ldate|log.Ltime)
}

func loggear(mensaje string) {
  ServiceLog.Output(0, mensaje)
}

func loggearError(mensaje string) {
  ErrorLog.Output(0, mensaje)
}

func clienteRedis() *redis.Client {
  // Necesito tener esto acá por utilizar el cliente como una var
  client := redis.NewClient(&redis.Options{
	  Addr: RedisHost + ":6379",
  })

  _, err := client.Ping().Result()
  if err != nil {
    fmt.Println("[ERROR] No se pudo conectar al servidor de Redis.")
    panic(err)
  }

  return client
}

func subscribir() {
  PubSubPlay = ClientePlay.Subscribe("interventions:play_audio_file")
  PubSubStop = ClienteStop.Subscribe("force-stop-broadcast")
}

func playLooper(){
  for {
    // Mensaje publicado
    mensaje, err := PubSubPlay.ReceiveMessage()
    if err != nil {
      loggearError(err.Error())
      panic(err)
    }
    loggear("[REDIS] " + mensaje.String())

    // El path del archivo está definido por firehouse_path y el payload del mensaje publicado
    // Por ejemplo: /firehouse/uploads/intervention_type/audio/1/01_archivo.mp3
    handler := exec.Command("cvlc", FirehousePath + mensaje.Payload,
                            "--play-and-exit",
		            "--sout='#transcode{vcodec=none,acodec=mp3}:udp{dst=" + BroadcastIP + ":8000, mux=raw, caching=10}'",
		            "--no-sout-rtp-sap",
		            "--no-sout-standard-sap",
		            "--ttl=1",
		            "--sout-keep",
		            "--sout-mux-caching=10")
    cmdString := strings.Join(handler.Args[:], " ")

    // Verifico si existe ya una instancia
    if VLCpid > 0 {
      loggear("[PLAYER] Se intentó iniciar una instancia de VLC pero ya existe por lo menos una.")
    } else {
      loggear("[PLAYER] Reproduciendo " + FirehousePath + mensaje.Payload + " con " + cmdString + ".")
      _, err := handler.CombinedOutput()
      if err != nil {
        loggearError(err.Error())
	panic(err)
      }
      VLCpid = handler.Process.Pid
    }
  }
}

func stopLooper(){
  for {
    // Mensaje publicado
    mensaje, err := PubSubStop.ReceiveMessage()
    if err != nil {
      loggearError(err.Error())
      panic(err)
    }
    loggear("[REDIS] " + mensaje.String())

    // Cierro todas las instancias
    if VLCpid > 0 {
      proceso, err := os.FindProcess(VLCpid)
      if err != nil {
        loggearError(err.Error())
      }
      err := proceso.Signal(syscall.SIGTERM)
      VLCpid = 0
    } else {
      loggear("[PLAYER] Se intentó cerrar una instancia de VLC pero no existe una disponible.")
    }
  }
}

func main() {

  var wg sync.WaitGroup
  wg.Add(2)

  loggear("Iniciando Servicio de Broadcast con VLC...")
  subscribir()

  go playLooper()
  go stopLooper()

  wg.Wait()
}
