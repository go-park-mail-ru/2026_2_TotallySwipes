package service

import "context"

// PhotoStorage хранит файлы фотографий отдельно от их метаданных в БД.
type PhotoStorage interface {
	// Save сохраняет уже проверенное изображение data с расширением ext
	Save(ctx context.Context, data []byte, ext string) (key string, err error)
	// Delete удаляет файл по ключу. Отсутствие файла считается успехом,
	Delete(ctx context.Context, key string) error
}

// PhotoURLProvider формирует ссылки для просмотра фотографий по ключам хранилища.
type PhotoURLProvider interface {
	// GetPhotoURL возвращает ссылку, которая может иметь ограниченный срок действия.
	// Формирование ссылки не гарантирует существование файла в хранилище.
	GetPhotoURL(ctx context.Context, key string) (string, error)
}
