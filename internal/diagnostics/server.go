package debug

import (
	"log/slog"
	"net"
	"net/http"
)

type Server struct {
	logger *slog.Logger
}

func New(logger *slog.Logger) *Server {
	return &Server{logger: logger}
}

func (s *Server) serve() {
	go func() {
		listener, err := net.Listen("tcp", "127.0.0.1:6060")
		if err != nil {
			s.logger.Error("something went wrong while establishing tcp connection with server")
			return
		}

		err = http.Serve(listener, s.route())
		if err != nil {
			s.logger.Error("")
			return
		}
	}()
}
