package common

const (
	SERVICE_CONFIG = "pydio.service.configs"

	SERVICE_ACL       = "pydio.service.acl"
	SERVICE_ROLE      = "pydio.service.role"
	SERVICE_USER      = "pydio.service.user"
	SERVICE_AUTH      = "pydio.service.auth"
	SERVICE_TREE      = "pydio.service.tree"
	SERVICE_WORKSPACE = "pydio.service.workspace"
	SERVICE_META      = "pydio.service.meta"
	SERVICE_SEARCH    = "pydio.service.search"
	SERVICE_ACTIVITY  = "pydio.service.activity"

	SERVICE_TIMER    = "pydio.service.timer"
	SERVICE_JOBS     = "pydio.service.jobs"
	SERVICE_VERSIONS = "pydio.service.versions"
	SERVICE_TASKS    = "pydio.service.tasks"
	SERVICE_DOCSTORE = "pydio.service.docstore"

	SERVICE_INDEX_     = "pydio.service.index."
	SERVICE_OBJECTS_   = "pydio.service.objects."
	SERVICE_SYNC_      = "pydio.service.sync."
	SERVICE_ENCRYPTION = "pydio.service.encryption"
	SERVICE_MAILER     = "pydio.service.mailer"

	SERVICE_API_NAMESPACE_ = "pydio.service.api."

	TOPIC_INDEX_CHANGES    = "topic.pydio.index.nodes.changes"
	TOPIC_TREE_CHANGES     = "topic.pydio.tree.nodes.changes"
	TOPIC_META_CHANGES     = "topic.pydio.meta.nodes.changes"
	TOPIC_TIMER_EVENT      = "topic.pydio.meta.timer.event"
	TOPIC_JOB_CONFIG_EVENT = "topic.pydio.jobconfig.event"
	TOPIC_JOB_TASK_EVENT   = "topic.pydio.jobconfig.event"

	META_NAMESPACE_OBJECT_SERVICE         = "pydio:meta-object-service-url"
	META_NAMESPACE_DATASOURCE_NAME        = "pydio:meta-data-source-name"
	META_NAMESPACE_DATASOURCE_PATH        = "pydio:meta-data-source-path"
	META_NAMESPACE_NODE_TEST_LOCAL_FOLDER = "pydio:test:local-folder-storage"

	PYDIO_THUMBSTORE_NAMESPACE        = "pydio-thumbstore"
	PYDIO_DOCSTORE_BINARIES_NAMESPACE = "pydio-binaries"
	PYDIO_VERSIONS_NAMESPACE = "versions-store"

	PYDIO_CONTEXT_USER_KEY = "X-Pydio-User"
)
