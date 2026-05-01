package diag

import (
	"fmt"
	"io"

	"github.com/setupproof/setupproof/internal/planning"
)

func EmitPlan(plan planning.Plan, stderr io.Writer) {
	for _, warning := range plan.Warnings {
		fmt.Fprintf(stderr, "warning: %s\n", warning)
	}
	for _, validationError := range plan.ValidationErrors {
		fmt.Fprintf(stderr, "error: %s\n", validationError)
	}
}
