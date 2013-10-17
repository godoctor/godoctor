package doctor

// Entrypoint for the refactoring engine.  This package enumerates the
// available refactorings and provides the a short name for each refactoring
// (which is used by tests, among other things).

var refactorings map[string]Refactoring

func init() {
	refactorings = map[string]Refactoring{
		"null":   new(NullRefactoring),
		"rename": new(RenameRefactoring),
	}
}

func GetAllRefactorings() map[string]Refactoring {
	return refactorings
}

func GetRefactoring(shortName string) Refactoring {
	return refactorings[shortName]
}
