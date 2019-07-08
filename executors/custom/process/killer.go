package process

type Killer interface {
	Terminate()
	ForceKill()
}
