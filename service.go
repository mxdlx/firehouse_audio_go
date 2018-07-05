package main

import (
  "bufio"
  "github.com/gomodule/redigo/redis"
  "io"
  "log"
  "os"
  "os/exec"
  "strconv"
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

func connRedis() redis.Conn {
  // Necesito tener esto acá por utilizar el cliente como una var
  c, err := redis.Dial("tcp", RedisHost + ":6379")
  if err != nil {
    loggearError("[REDIS] No se pudo conectar al servidor de Redis.")
    panic(err)
  }
  //defer c.Close()

  return c
}

func startBroadcast(){
  c := connRedis()
  loggear("[PLAYER] Estoy en startBroadcast!")
  loggear("[REDIS] Publicando mensaje en start-broadcast.")
  time.Sleep(2 * time.Second)
  c.Send("PUBLISH", "start-broadcast", "Iniciar Broadcast")
  defer c.Close()
}

func stopBroadcast(){
  c := connRedis()
  loggear("[PLAYER] Estoy en stopBroadcast!")
  loggear("[REDIS] Publicando mensaje en stop-broadcast.")
  time.Sleep(2 * time.Second)
  c.Send("PUBLISH", "stop-broadcast","Detener Broadcast")
  defer c.Close()
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

  StdinVLC, _ = handler.StdinPipe()
  StdoutVLC, _ = handler.StdoutPipe()

  handler.Start()
}

func playLooper(){
  c := connRedis()
  PubSubPlay := redis.PubSubConn{Conn: c}
  PubSubPlay.Subscribe("interventions:play_audio_file")
  for {
    switch m := PubSubPlay.Receive().(type) {
      case redis.Message:
        payload := string(m.Data[:])
        loggear("[REDIS] Mensaje publicado: " + payload + ".")

        // URI del archivo para VLC
        file := "file://" + FirehousePath + payload

        startBroadcast()

        loggear("[PLAYER] Agregando " + FirehousePath + payload + " a la playlist de VLC.")
        loggear("[PLAYER] Intentando reproducir " + FirehousePath + payload)
        io.WriteString(StdinVLC, "add " + file + "\n")
    }
  }
  defer c.Close()
}

func stopLooper(){
  c := connRedis()
  PubSubStop := redis.PubSubConn{Conn: c}
  PubSubStop.Subscribe("stop-broadcast", "force-stop-broadcast")
  for {
    switch m := PubSubStop.Receive().(type) {
      case redis.Message:
        payload := string(m.Data[:])
        loggear("[REDIS] Mensaje publicado: " + payload + ".")

        // bufio al rescate
        bufaio := bufio.NewScanner(StdoutVLC)

        // Hacer is_playing
        io.WriteString(StdinVLC, "clear\n")
        io.WriteString(StdinVLC, "is_playing\n")

        for {
          loggear("[BUFIO] Puedo hacer Scan(): " + strconv.FormatBool(bufaio.Scan()))
          loggear("[BUFIO] Linea: " + bufaio.Text())

          if ( bufaio.Text() == "0" || bufaio.Text() == "1" ) {
            break
          }
        }

        if bufaio.Text() == "1" {
          loggear("[BUFIO] Linea: " + bufaio.Text())
          loggear("[STOPPER] Deteniendo playlist de VLC.")
          io.WriteString(StdinVLC, "stop\n")
          stopBroadcast()
        }
    }
  }
  defer c.Close()
}

func main() {

  var wg sync.WaitGroup
  wg.Add(3)

  loggear("Iniciando Servicio de Broadcast con VLC...")

  go vlcLoader()
  go playLooper()
  go stopLooper()

  wg.Wait()
}
