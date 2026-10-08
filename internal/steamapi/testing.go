package steamapi

// SetKeyForTest installs a key in memory only, so tests in other packages
// don't write to the user's config directory.
func SetKeyForTest(c *Client, key string) {
	c.mu.Lock()
	c.key, c.path = key, ""
	c.mu.Unlock()
}
