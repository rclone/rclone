package src

// EmptyTrash will permanently delete all trashed files/folders from Yandex Disk
func (c *Client) EmptyTrash() error {
	fullURL := RootAddr
	fullURL += "/v1/disk/trash/resources"

	return c.PerformDelete(fullURL)
}
