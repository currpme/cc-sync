package model

type AppConfig struct {
	WebDAV   WebDAVConfig
	Remote   RemoteConfig
	Sync     SyncConfig
	Scan     ScanConfig
	Conflict ConflictConfig
}

type WebDAVConfig struct {
	URL         string
	Username    string
	Password    string
	PasswordCmd string
}

type RemoteConfig struct {
	Root string
}

type SyncConfig struct {
	ManageConfig        bool
	ManageInstructions  bool
	ManageUserSkills    bool
	ManageProjectSkills bool
	ManageMCP           bool
	DefaultMode         string
	AllowDelete         bool
}

type ScanConfig struct {
	ProjectRoots []string
}

type ConflictConfig struct {
	DefaultResolution string
}

type ItemType string

const (
	ItemConfig       ItemType = "config"
	ItemInstruction  ItemType = "instruction"
	ItemUserSkill    ItemType = "user_skill"
	ItemProjectSkill ItemType = "project_skill"
	ItemMCP          ItemType = "mcp"
)

type ManagedItem struct {
	Tool       string   `json:"tool"`
	Type       ItemType `json:"type"`
	ID         string   `json:"id"`
	RelPath    string   `json:"rel_path"`
	ProjectRef string   `json:"project_ref,omitempty"`
	Content    []byte   `json:"-"`
	Hash       string   `json:"hash"`
}

type Snapshot struct {
	Tool  string        `json:"tool"`
	Items []ManagedItem `json:"items"`
}
