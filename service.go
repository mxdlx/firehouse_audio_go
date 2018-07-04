package main

import (
  "github.com/go-redis/redis"
  "log"
  "os"
  "os/exec"
  "strconv"
  "strings"
  "sync"
  "syscall"
  "time"
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
    loggearError("[REDIS] No se pudo conectar al servidor de Redis.")
    panic(err)
  }

  return client
}

func subscribir() {
  PubSubPlay = ClientePlay.Subscribe("interventions:play_audio_file")
  PubSubStop = ClienteStop.Subscribe("stop-broadcast, force-stop-broadcast")
}

func startBroadcast(){
  loggear("[REDIS] Publicando mensaje en start-broadcast")
  err := ClientePlay.Publish("start-broadcast", "Iniciar Broadcast").Err()
  if err != nil {
    loggearError("[PLAYER] Hubo un error al publicar un mensaje en start-broadcast.")
  }
}

func stopBroadcast(){
  loggear("[REDIS] Publicando mensaje en stop-broadcast")
  time.Sleep(2 * time.Second)
  err := ClienteStop.Publish("stop-broadcast", "Detener Broadcast").Err()
  if err != nil {
    loggearError("[STOPPER] Hubo un error al publicar un mensaje en stop-broadcast.")
  }
}

func playLooper(){
  for {
    // Mensaje publicado
    mensaje, err := PubSubPlay.ReceiveMessage()
    if err != nil {
      loggearError("[REDIS] Hubo en error en la recepción de un mensaje publicado. Sistema dice: " + err.Error() + ".")
      panic(err)
    }
    loggear("[REDIS] " + mensaje.String())

    // El path del archivo está definido por firehouse_path y el payload del mensaje publicado
    // Por ejemplo: /firehouse/uploads/intervention_type/audio/1/01_archivo.mp3
    var file, sout string
    file = FirehousePath + mensaje.Payload
    sout = "--sout=#transcode{vcodec=none,acodec=mp3}:udp{dst=" + BroadcastIP + ":8000,caching=10,mux=raw}"

    handler := exec.Command("cvlc", file,
                            "--play-and-exit",
		            sout,
		            "--no-sout-rtp-sap",
		            "--no-sout-standard-sap",
		            "--ttl=1",
		            "--sout-keep",
		            "--sout-mux-caching=10")

    // Verifico si existe ya una instancia
    if VLCpid > 0 {
      loggear("[PLAYER] Se intentó iniciar una instancia de VLC pero ya existe por lo menos una.")
    } else {
      startBroadcast()
      err := handler.Start()
      loggear("[PLAYER] Reproduciendo " + file + " con " + strings.Join(handler.Args[:], " ") + ".")
      if err != nil {
        loggearError("[PLAYER] Hubo un error en la ejecución de VLC. Sistema dice: " + err.Error() + ".")
	panic(err)
      }
      loggear("[PLAYER] El PID de VLC es: " + strconv.Itoa(handler.Process.Pid))
      VLCpid = handler.Process.Pid
    }
  }
}

func stopLooper(){
  for {
    // Mensaje publicado
    mensaje, err := PubSubStop.ReceiveMessage()
    if err != nil {
      loggearError("[REDIS] Hubo en error en la recepción de un mensaje publicado. Sistema dice: " + err.Error() + ".")
      panic(err)
    }
    loggear("[REDIS] " + mensaje.String())

    // Cierro todas las instancias
    if VLCpid > 0 {
      loggear("[STOPPER] Intentado cerrar instancia de VLC con PID: " + strconv.Itoa(VLCpid))
      proceso, errFP := os.FindProcess(VLCpid)
      if errFP != nil {
        loggearError("[STOPPER] No se encontró un proceso para terminar. Sistema dice: " + errFP.Error() + ".")
	VLCpid = 0
      } else {
        errN := proceso.Signal(syscall.SIGTERM)
        if errN != nil {
	  loggearError("No se pudo enviar la señal SIGTERM. Sistema dice: " + errN.Error() + ".")
	  VLCpid = 0
	} else {
	  loggear("Se envío la señal SIGTERM.")
          VLCpid = 0
	}
      }
      stopBroadcast()
    } else {
      loggear("[STOPPER] No hay instancias para terminar.")
      VLCpid = 0
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
