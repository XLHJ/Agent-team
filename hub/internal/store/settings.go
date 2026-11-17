package store

// Simple key/value settings used by hub-side integrations (e.g. the kitesurf
// bridge endpoint configured from the panel).

// SettingGet returns the stored value for key ("", nil) when absent.
func (d *DB) SettingGet(key string) (string, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	var v string
	err := d.db.QueryRow(`SELECT value FROM app_settings WHERE key = ?`, key).Scan(&v)
	if err != nil {
		return "", nil // absent key is not an error
	}
	return v, nil
}

// SettingSet upserts a setting.
func (d *DB) SettingSet(key, value string) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	_, err := d.db.Exec(
		`INSERT INTO app_settings(key,value) VALUES(?,?)
 ON CONFLICT(key) DO UPDATE SET value=excluded.value`, key, value)
	return err
}
