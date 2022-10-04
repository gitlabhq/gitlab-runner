package networks

//go:generate mockery --name=debugLogger --inpackage
type debugLogger interface {
	Debugln(args ...interface{})
}
