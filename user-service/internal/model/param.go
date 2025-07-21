package model

type LoginParam struct {
	UserName string
	Pwd      string
}

type SignupParam struct {
	UserName string
	Pwd      string
	RePwd    string
	Tel      string
}
