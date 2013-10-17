package doctor

// TODO: Document engine

var refactorings map[string]Refactoring

func init() {
	refactorings = map[string]Refactoring{
		"rename": new(RenameRefactoring)}
}

func GetAllRefactorings() map[string]Refactoring {
	return refactorings
}

func GetRefactoring(shortName string) Refactoring {
	return refactorings[shortName]
}
