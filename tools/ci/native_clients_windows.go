package main

// supplyInUse makes no process inventory on Windows: the command-line listing
// that the POSIX check reads has no counterpart here, so retirement removes the
// supply directly.
func supplyInUse(_ string, _ outputRunner) error {
	return nil
}
