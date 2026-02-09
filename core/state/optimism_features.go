package state

// MakeSinglethreaded is a no-op placeholder for cannon compatibility.
// This function is called by op-program to disable concurrent state operations
// when running in the cannon fault proof VM environment.
// The actual singlethreaded mode implementation (workers.go) is not yet synced
// from upstream op-geth, so this remains a stub for API compatibility.
func (s *StateDB) MakeSinglethreaded() {
	// no-op
}
