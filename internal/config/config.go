package config

import (
	"os"

	"arbitrage-monitor/pkg/models"

	"gopkg.in/yaml.v3"
)

// LoadConfig читает и парсит YAML конфигурационный файл
// Параметры:
//
//	path - путь к файлу config.yaml
//
// Возвращает:
//
//	*models.Config - распарсенная конфигурация
//	error - ошибка при чтении файла или парсинге
func LoadConfig(path string) (*models.Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var cfg models.Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}
