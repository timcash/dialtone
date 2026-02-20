package test

type Registry struct {
	Steps []Step
}

func NewRegistry() *Registry {
	return &Registry{
		Steps: make([]Step, 0),
	}
}

func (r *Registry) Add(step Step) {
	r.Steps = append(r.Steps, step)
}

func (r *Registry) Run(opts SuiteOptions) error {
	return RunSuite(opts, r.Steps)
}
