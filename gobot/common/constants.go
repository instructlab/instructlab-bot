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
