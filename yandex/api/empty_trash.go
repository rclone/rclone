package src

// EmptyTrash will permanently delete all trashed files/folders from Yandex Disk
func (c *Client) EmptyTrash() error {
	fullURL := RootAddr
	fullURL += "/v1/disk/trash/resources"

	if err := c.PerformDelete(fullURL); err != nil {
		return err
	}

	return nil
}
