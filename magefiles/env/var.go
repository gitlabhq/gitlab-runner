package env

type Variable struct {
	Key      string
	Fallback string
	Default  string
}

type VariableBundle interface {
	Variables() []Variable
}

func (v Variable) Variables() []Variable {
	return []Variable{v}
}

type Variables []Variable

func (v Variables) Variables() []Variable {
	return v
}

func New(key string) Variable {
	return Variable{
		Key: key,
	}
}

func NewDefault(key, def string) Variable {
	return Variable{
		Key:     key,
		Default: def,
	}
}

func NewFallbackOrDefault(key, fallback, def string) Variable {
	return Variable{
		Key:      key,
		Fallback: fallback,
		Default:  def,
	}
}
