package service_test

//go:generate go run go.uber.org/mock/mockgen -source=../chat_service.go -destination=chat_mocks_test.go -package=service_test
//go:generate go run go.uber.org/mock/mockgen -source=../chat_events_service.go -destination=chat_events_mocks_test.go -package=service_test
