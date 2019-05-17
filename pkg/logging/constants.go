package logging

// Service types
const (
	CreateService int = iota
	OrderService
	SyncService
	ValidateService
)

// serviceType maps integer service types to human readable names
var serviceType = map[int]string{
	0: "create",
	1: "order",
	2: "sync",
	3: "validate",
}
