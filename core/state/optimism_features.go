package state

// MakeSinglethreaded sets the StateDB to singlethreaded mode, disabling any
// concurrent state operations. This is useful for testing and debugging.

func (s *StateDB) MakeSinglethreaded() {
	s.singlethreaded = true
}
