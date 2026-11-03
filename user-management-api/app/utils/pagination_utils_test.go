package utils_test

import (
	"testing"

	"github.com/SarawinT/user-management-api/app/utils"
)

func TestPaginate(t *testing.T) {
	tests := []struct {
		page, size, wantOffset, wantLimit int
	}{
		{0, 0, 0, utils.DefaultPageSize},
		{2, 10, 20, 10},
		{1, 1000, utils.MaxPageSize, utils.MaxPageSize},
		{-3, 10, 0, 10},
	}
	for _, tt := range tests {
		offset, limit := utils.Paginate(tt.page, tt.size)
		if offset != tt.wantOffset || limit != tt.wantLimit {
			t.Errorf("Paginate(%d, %d) = (%d, %d), want (%d, %d)",
				tt.page, tt.size, offset, limit, tt.wantOffset, tt.wantLimit)
		}
	}
}
