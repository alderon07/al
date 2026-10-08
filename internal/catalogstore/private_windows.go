package catalogstore

import "errors"

func ReadPrivate(path string, destination any) error { return readPrivate(path, destination, nil) }
func readPrivate(path string, destination any, observed func()) error {
	return errors.New("private catalog state observation is unsupported on Windows; use Linux or WSL")
}

func ReadPrivateBytes(path string, limit int64) ([]byte, error) {
	return nil, errors.New("private catalog state observation is unsupported on Windows; use Linux or WSL")
}
