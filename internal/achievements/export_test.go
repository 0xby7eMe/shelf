package achievements

import "shelf/internal/steamapi"

// setTestKey gives a client a key without writing the user's config file.
func setTestKey(c *steamapi.Client) { steamapi.SetKeyForTest(c, "KEY") }
