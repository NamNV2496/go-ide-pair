package model

type ProgrammingLanguage int

const (
	C ProgrammingLanguage = iota
	Cpp
	Java
	Python3
)

type SourceCode struct {
	Name     string              `json:"name" valid:"length(0|128)"`
	Language ProgrammingLanguage `json:"language" valid:"range(0|4)"`
	Content  string              `json:"content" valid:"length(0|8192)"`
	Input    string              `json:"input" valid:"length(0|8192),optional"`
}
