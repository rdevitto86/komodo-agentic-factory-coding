//go:generate mockgen -source=foo.go

func Foo() {
	bar() //nolint:errcheck
}
