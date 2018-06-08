# Broadcast de audio (RTP sobre UDP) con VLC y Go
Esto es una adaptación en Go de [shelvak/firehouse\_audio](https://github.com/shelvak/firehouse_audio). Es una implementación específica para un sistema de activación de alarmas en un cuartel de Bomberos. Distintas aplicaciones se comunican mediante Redis y dependiendo del mensaje publicado se reproduce o se detiene el streaming con VLC, literalmente, se hace exec y kill del comando.

Como estos servicios corren en contenedores, me pareció una buena idea hacer el intento con Go que no es tan atractivo como Ruby pero tiene en este caso cero dependencias y (aún sin evaluar a fondo) menor utilización de recursos.
