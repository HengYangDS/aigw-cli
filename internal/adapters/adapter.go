package adapters

type ProcessPlan struct {
	Executable string
	Args       []string
	Env        []string
	Stdin      string
}
