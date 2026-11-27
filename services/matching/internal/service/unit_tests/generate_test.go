package service

//go:generate go run go.uber.org/mock/mockgen -source=../event_service.go -destination=event_mocks_test.go -package=service
//go:generate go run go.uber.org/mock/mockgen -source=../match_service.go -destination=match_mocks_test.go -package=service
//go:generate go run go.uber.org/mock/mockgen -source=../feed_service.go -destination=feed_mocks_test.go -package=service
//go:generate go run go.uber.org/mock/mockgen -source=../swipe_service.go -destination=swipe_mocks_test.go -package=service
