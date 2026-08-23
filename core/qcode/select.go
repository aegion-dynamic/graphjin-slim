package qcode

// GetInternalArg returns the internal argument registered under name.
// Internal args are compiler bookkeeping (e.g. cursor and connect
// payloads), not user-visible arguments.
func (sel *Select) GetInternalArg(name string) (Arg, bool) {
	var arg Arg
	for _, v := range sel.IArgs {
		if v.Name == name {
			return v, true
		}
	}
	return arg, false
}
