//go:generate go tool go.uber.org/mock/mockgen -source=../domain/repositories.go -destination=repositories_mock.go -package=mock
//go:generate go tool go.uber.org/mock/mockgen -source=../domain/services.go -destination=services_mock.go -package=mock

package mock
