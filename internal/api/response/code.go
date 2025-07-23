package response

// 响应码常量
const (
	// 成功
	SUCCESS = "SUCCESS"

	// 客户端错误
	INVALID_REQUEST = "INVALID_REQUEST" // 无效的请求参数
	INVALID_TASK_ID = "INVALID_TASK_ID" // 无效的任务ID
	TASK_NOT_FOUND  = "TASK_NOT_FOUND"  // 任务未找到

	// 视频相关错误
	VIDEO_INFO_ERROR = "VIDEO_INFO_ERROR" // 获取视频信息失败
	DOWNLOAD_ERROR   = "DOWNLOAD_ERROR"   // 下载视频失败

	// 服务器错误
	SERVER_ERROR = "SERVER_ERROR" // 服务器内部错误
)

// GetMessage 根据响应码获取对应的消息
func GetMessage(code string) string {
	switch code {
	case SUCCESS:
		return "操作成功"
	case INVALID_REQUEST:
		return "无效的请求参数"
	case INVALID_TASK_ID:
		return "无效的任务ID"
	case TASK_NOT_FOUND:
		return "任务未找到"
	case VIDEO_INFO_ERROR:
		return "获取视频信息失败"
	case DOWNLOAD_ERROR:
		return "下载视频失败"
	case SERVER_ERROR:
		return "服务器内部错误"
	default:
		return "未知错误"
	}
}