package common

const (
	RepoName = "taxonomy"

	CheckComplete      = "completed"
	CheckQueued        = "queued"
	CheckInProgress    = "in_progress"
	CheckStatusSuccess = "success"
	CheckStatusFailure = "failure"
	CheckStatusError   = "error"
	CheckStatusPending = "pending"

	BotReadyStatus    = "InstructLab Bot"
	BotReadyStatusMsg = "InstructLab bot is ready to assist!!"

	PrecheckCheck      = "Precheck Check"
	GenerateLocalCheck = "Generate Local Check"
	GenerateSDGCheck   = "Generate SDG Check"

	PrecheckStatus      = "Precheck Status"
	GenerateLocalStatus = "Generate Local Status"
	GenerateSDGStatus   = "Generate SDG Status"

	InstructLabBotUrl = "https://github.com/instructlab/instructlab-bot"
)

// Not sure we need all this, keeping for now until tested with just name
const (
	TrainingDataApproveCheck  = "Training Data Approval Check"
	TrainingDataApproveStatus = "Training Data Approval Status"
	// TrainingDataAprroveLabelID          int64 = 6964465752
	// TrainingDataApproveLabelNodeId            = "LA_kwDOLf49gM8AAAABnx1QWA"
	// TrainingDataApproveLabelURL               = "https://api.github.com/repos/instructlab/instructlab-bot/labels/training-data-approved"
	TrainingDataApproveLabelName = "training-data-approved"
	// TrainingDataApproveLabelColor             = "18F02A"
	// TrainintDataApproveLabelDefault           = false
	// TrainingDataApproveLabelDescription       = "generated seed data approved, you may run ilab train."
)

const (
	RedisKeyJobs           = "jobs"
	RedisKeyPRNumber       = "pr_number"
	RedisKeyPRSHA          = "pr_sha"
	RedisKeyAuthor         = "author"
	RedisKeyInstallationID = "installation_id"
	RedisKeyRepoOwner      = "repo_owner"
	RedisKeyRepoName       = "repo_name"
	RedisKeyJobType        = "job_type"
	RedisKeyErrors         = "errors"
	RedisKeyRequestTime    = "request_time"
	RedisKeyDuration       = "duration"
	RedisKeyStatus         = "status"
)
