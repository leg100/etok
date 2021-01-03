package controllers

type state struct {
	Serial  int
	Outputs map[string]output
}

type output struct {
	Type  string
	Value string
}
