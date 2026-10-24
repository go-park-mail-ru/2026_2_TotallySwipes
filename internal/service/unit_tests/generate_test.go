package service

//go:generate go run go.uber.org/mock/mockgen -destination=service_mocks_test.go -package=service dating-app/internal/service ProfileRepository,PhotoStorage,PhotoURLProvider,CompatibilityService,UserRepository,ProfileMissingChecker,SessionRepository,PasswordHasher,AccessTokenIssuer,FilterRepository,PsychoTestRepository,ProfileReader,SwipeRepository

//go:generate go run go.uber.org/mock/mockgen -source=../location_service.go -destination=location_mocks_test.go -package=service
