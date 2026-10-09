package httpx

import "github.com/gofiber/fiber/v2"

// Pagination reads ?page and ?page_size, clamping them to sane bounds.
func Pagination(c *fiber.Ctx, defaultSize, maxSize int) (page, size int) {
	page = max(c.QueryInt("page", 1), 1)
	size = min(max(c.QueryInt("page_size", defaultSize), 1), maxSize)
	return page, size
}
