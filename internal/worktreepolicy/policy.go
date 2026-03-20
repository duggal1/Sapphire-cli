package worktreepolicy

type Policy string

const (
	SharedRepo Policy = "shared_repo"
	Auto       Policy = "auto"
	Isolated   Policy = "isolated_worktree"
)

func Default() Policy {
	return SharedRepo
}

func Normalize(policy Policy) Policy {
	switch policy {
	case SharedRepo, Auto, Isolated:
		return policy
	default:
		return Default()
	}
}

func (p Policy) IsValid() bool {
	switch p {
	case SharedRepo, Auto, Isolated:
		return true
	default:
		return false
	}
}

func (p Policy) Title() string {
	switch Normalize(p) {
	case Auto:
		return "Auto"
	case Isolated:
		return "Worktree"
	default:
		return "Repo"
	}
}

func (p Policy) FooterLabel() string {
	switch Normalize(p) {
	case Auto:
		return "WT AUTO"
	case Isolated:
		return "WT ON"
	default:
		return ""
	}
}

func (p Policy) Cycle() Policy {
	switch Normalize(p) {
	case SharedRepo:
		return Auto
	case Auto:
		return Isolated
	default:
		return SharedRepo
	}
}
