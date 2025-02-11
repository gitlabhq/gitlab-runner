package api

//go:generate mockery --name=InitGracefulShutdownRequest --inpackage --with-expecter
type InitGracefulShutdownRequest interface {
	ShutdownCallbackDef() ShutdownCallbackDef
}

type defaultInitGracefulShutdownRequest struct {
	shutdownCallbackDef ShutdownCallbackDef
}

func NewInitGracefulShutdownRequest(shutdownCallbackDef ShutdownCallbackDef) InitGracefulShutdownRequest {
	return &defaultInitGracefulShutdownRequest{
		shutdownCallbackDef: shutdownCallbackDef,
	}
}

func (d *defaultInitGracefulShutdownRequest) ShutdownCallbackDef() ShutdownCallbackDef {
	return d.shutdownCallbackDef
}
