package qcode

// GetQTypeByName maps a language-level operation name to its QType.
func GetQTypeByName(t string) QType {
	switch t {
	case "query":
		return QTQuery
	case "subscription":
		return QTSubscription
	case "mutation":
		return QTMutation
	default:
		return QTUnknown
	}
}
