package launcher

import (
	"fmt"

	"github.com/leg100/stok/util"
)

// Generate unique name shared by run and configmap resources (and run ctrl will spawn a
// pod with this name, too)
var GenerateName = generateName

func generateName() string {
	return fmt.Sprintf("run-%s", util.GenerateRandomString(5))
}
