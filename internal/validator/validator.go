package validator

type Validator struct {
	Errors map[string]string
}

func New() *Validator {
	return &Validator{Errors: make(map[string]string)}
}

func (v *Validator) Valid() bool {
	return len(v.Errors) == 0
}

func (v *Validator) AddError(key string, value string) {
	if _, exist := v.Errors[key]; !exist {
		v.Errors[key] = value
	}
}

func (v *Validator) Check(ok bool, key string, value string) {
	if !ok {
		v.AddError(key, value)
	}
}
