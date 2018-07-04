package main

import (
  "bytes"
  "github.com/go-redis/redis"
  "io"
  "log"
  "os"
  "os/exec"
  "sync"
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
  VLCLogPath = LogPath + "/vlc.log"
  StdinVLC io.WriteCloser
  StdoutVLC io.ReadCloser
  ClientePlay = clienteRedis()
  ClienteStop = clienteRedis()
  PubSubPlay *redis.PubSub
  PubSubStop *redis.PubSub
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
  PubSubStop = ClienteStop.Subscribe("force-stop-broadcast")
}

func startBroadcast(){
  loggear("[REDIS] Publicando mensaje en start-broadcast.")
  err := ClientePlay.Publish("start-broadcast", "Iniciar Broadcast").Err()
  if err != nil {
    loggearError("[PLAYER] Hubo un error al publicar un mensaje en start-broadcast.")
  }
}

func stopBroadcast(){
  loggear("[REDIS] Publicando mensaje en stop-broadcast.")
  time.Sleep(2 * time.Second)
  err := ClienteStop.Publish("stop-broadcast", "Detener Broadcast").Err()
  if err != nil {
    loggearError("[STOPPER] Hubo un error al publicar un mensaje en stop-broadcast.")
  }
}

func vlcLoader(){
  loggear("[PLAYER] Iniciando instancia de VLC.")
  sout := "--sout=#transcode{vcodec=none,acodec=mp3}:udp{dst=" + BroadcastIP + ":8000,caching=10,mux=raw}"
  handler := exec.Command("cvlc",
                          "-I",
			  "oldrc",
			  "--rc-fake-tty",
			  "--file-logging",
			  "--logfile",
			  VLCLogPath,
			  "--log-verbose",
			  "2",
			  sout,
			  "--no-sout-rtp-sap",
			  "--no-sout-standard-sap",
			  "--ttl=1",
			  "--sout-keep",
			  "--sout-mux-caching=10")

  // Necesito compartir StdinVLC
  var err1, err2 error
  StdinVLC, err1 = handler.StdinPipe()
  StdoutVLC, err2 = handler.StdoutPipe()

  handler.Start()

  if err != nil {
    loggearError("[PLAYER] Hubo un error en la ejecución de VLC. Sistema dice: " + err.Error() + ".")
    panic(err)
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
    loggear("[REDIS] Mensaje publicado: " + mensaje.String() + ".")

    // URI del archivo para VLC
    file := "file:// " + FirehousePath + mensaje.Payload

    startBroadcast()

    loggear("[PLAYER] Agregando " + FirehousePath + mensaje.Payload + " a la playlist de VLC.")
    loggear("[PLAYER] Intentando reproducir " + FirehousePath + mensaje.Payload)
    io.WriteString(StdinVLC, "add " + file + "\n")
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
    loggear("[REDIS] Mensaje publicado: " + mensaje.String() + ".")

    io.WriteString(StdinVLC, "is_playing\n")
    buf := new(bytes.Buffer)
    buf.ReadFrom(StdoutVLC)

    if buf.String() == "1" {
      loggear("[STOPPER] Deteniendo playlist de VLC.")
      io.WriteString(StdinVLC, "stop\n")
      stopBroadcast()
    } else {
      loggear("[STOPPER] No hay elementos en reproduccion.")
    }
  }
}

func main() {

  var wg sync.WaitGroup
  wg.Add(3)

  loggear("Iniciando Servicio de Broadcast con VLC...")
  subscribir()

  go vlcLoader()
  go playLooper()
  go stopLooper()

  wg.Wait()
}
