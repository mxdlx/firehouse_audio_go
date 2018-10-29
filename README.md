# Broadcast de audio (RTP sobre UDP) con VLC y Go
Esto es una adaptación en Go de [shelvak/firehouse\_audio](https://github.com/shelvak/firehouse_audio). Es una implementación específica para un sistema de activación de alarmas en un cuartel de Bomberos. Distintas aplicaciones se comunican mediante Redis y dependiendo del mensaje publicado se reproduce o se detiene el streaming con VLC, literalmente, se hace exec y kill del comando.

Como estos servicios corren en contenedores, me pareció una buena idea hacer el intento con Go que no es tan atractivo como Ruby pero tiene en este caso cero dependencias y (aún sin evaluar a fondo) menor utilización de recursos.

### How-To

```bash
# Primero es necesario setear REDIS_HOST y BROADCAST_IP en docker-compose.yml con la IP del host y del broadcast del segmento de red donde se trabaje.

# Buildear la imagen con el servicio de VLC
$ sudo docker-compose build

# Correr el compose
$ sudo docker-compose up
```

### Redis Commander
Esta instancia levanta un cliente web de Redis en `http://127.0.0.1:8081`. Para más información sobre la utilización de este producto ver el [repositorio oficial](https://github.com/joeferner/redis-commander).
Este cliente incorpora una consola en la aplicación web. Los comandos para utilizar este repositorio son:

```bash
# Ver los canales disponibles
$ pubsub channels *
1) stop-broadcast
2) interventions:play_audio_file

# Reproducir audio
$ publish interventions:play_audio_file orbit.mp3

# Detener audio
$ publish stop-broadcast stop
```

### Reproducción de audio
El audio reproducido puede ser escuchado con VLC, abriendo un volcado de red y estableciendo como dirección la URL `udp://0.0.0.0:8000`.<br />
Para reproducir otros archivos de audio, es necesario colocarlos en el directorio firehouse de este repositorio. Luego utilizar su nombre como se muestra en el ejemplo anterior.