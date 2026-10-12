package board

// Section names of ticket and workstream files (board-format §Ticket,
// §Workstream). Code outside board names sections only through these.
const (
	SectionDescription  = "Description"
	SectionCriteria     = "Acceptance Criteria"
	SectionTestPlan     = "Test Plan"
	SectionReproduction = "Reproduction"
	SectionContext      = "Context"
	SectionNotes        = "Notes"
	SectionHandoff      = "Handoff"

	SectionGoal            = "Goal"
	SectionScope           = "Scope"
	SectionSuccessCriteria = "Success Criteria"
)

// ValidSlug reports a workstream slug: lower-case letters, digits, '.',
// '_' and '-', starting with a letter or digit, at most 100 characters.
// It is the one rule for slugs, whoever names one.
func ValidSlug(slug string) bool { return slugPattern.MatchString(slug) }
