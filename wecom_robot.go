package utility

import (
	"bytes"
	"encoding/json"
	"fmt"
	log "github.com/sirupsen/logrus"
	"io"
	"net/http"
	"runtime/debug"
	"time"
)

type MsgContent struct {
	Content string `json:"content"`
}

type WeComRobotMsg struct {
	MsgType string     `json:"msgtype"`
	Text    MsgContent `json:"text"`
}

type MarkDownMsg struct {
	MsgType  string     `json:"msgtype"`
	MarkDown MsgContent `json:"markdown"`
}

type WeComMsg struct {
	WeComUrl string
	AppName  string
	Msg      string
}

var weComServerUrl string
var appTargetName string
var weComRobotNum int
var weComRobotMsg = make(chan *WeComMsg) // 临时make一个出来，以免sendMsg时为空，导致crash

func InitWeComRobot(
	serverUrl string,
	serverName string,
	robotNum int,
	msgBufferLen int,
) {
	weComServerUrl = serverUrl
	appTargetName = serverName
	weComRobotNum = robotNum
	weComRobotMsg = make(chan *WeComMsg, msgBufferLen)

	for i := 0; i < weComRobotNum; i++ {
		go weComRobotAction()
	}
}

func StopWeComRobot() {
	//close(weComRobotMsg)
}

func JoinWeComRobot() {

}

func weComRobotAction() {
	defer func() {
		if err := recover(); err != nil {
			log.Error(err, string(debug.Stack()))
		}
	}()

	for msg := range weComRobotMsg {
		postWeComMsg(
			msg.WeComUrl,
			"markdown",
			msg.Msg,
		)
	}
}

func SendDebugMsgToWeComRobot(debugMsg string) {
	newMsg := &WeComMsg{
		WeComUrl: weComServerUrl,
		AppName:  appTargetName,
		Msg: fmt.Sprintf(`server : <font color="debug">%s</font>
					>time : <font color="comment">%s</font>
					>debug : <font color="comment">%s</font>`,
			appTargetName,
			time.Now().Format("2006-01-02 15:04:05"),
			debugMsg,
		),
	}

	select {
	case weComRobotMsg <- newMsg:
	default:
		return
	}
}

func SendDebugMsgToWeComRobotWithUrl(url string, debugString string) {
	newMsg := &WeComMsg{
		WeComUrl: url,
		AppName:  appTargetName,
		Msg: fmt.Sprintf(`server : <font color="debug">%s</font>
					>time : <font color="comment">%s</font>
					>debug : <font color="comment">%s</font>`,
			appTargetName,
			time.Now().Format("2006-01-02 15:04:05"),
			debugString,
		),
	}

	select {
	case weComRobotMsg <- newMsg:
	default:
		return
	}
}

func SendInfoMsgToWeComRobot(debugMsg string) {
	newMsg := &WeComMsg{
		WeComUrl: weComServerUrl,
		AppName:  appTargetName,
		Msg: fmt.Sprintf(`server : <font color="debug">%s</font>
					>time : <font color="comment">%s</font>
					>info : <font color="comment">%s</font>`,
			appTargetName,
			time.Now().Format("2006-01-02 15:04:05"),
			debugMsg,
		),
	}

	select {
	case weComRobotMsg <- newMsg:
	default:
		return
	}
}

func SendWarningMsgToWeComRobot(warningMsg string) {
	newMsg := &WeComMsg{
		WeComUrl: weComServerUrl,
		AppName:  appTargetName,
		Msg: fmt.Sprintf(`server : <font color="debug">%s</font>
					>time : <font color="comment">%s</font>
					>warning : <font color="comment">%s</font>`,
			appTargetName,
			time.Now().Format("2006-01-02 15:04:05"),
			warningMsg,
		),
	}

	select {
	case weComRobotMsg <- newMsg:
	default:
		return
	}
}

func SendErrorMsgToWeComRobot(errorMsg string) {
	newMsg := &WeComMsg{
		WeComUrl: weComServerUrl,
		AppName:  appTargetName,
		Msg: fmt.Sprintf(`server : <font color="error">%s</font>
					>time : <font color="comment">%s</font>
					>error : <font color="comment">%s</font>`,
			appTargetName,
			time.Now().Format("2006-01-02 15:04:05"),
			errorMsg,
		),
	}

	select {
	case weComRobotMsg <- newMsg:
	default:
		return
	}
}

func postWeComMsg(url string, msgType string, msg string) {
	defer func() {
		if err := recover(); err != nil {
			log.Error("postWeComMsg err: ", err, string(debug.Stack()))
		}
	}()

	var postData interface{}

	switch msgType {
	case "text":
		postData = WeComRobotMsg{
			MsgType: msgType,
			Text: MsgContent{
				Content: msg,
			},
		}
		break
	case "markdown":
		postData = MarkDownMsg{
			MsgType: msgType,
			MarkDown: MsgContent{
				Content: msg,
			},
		}
		break
	default:
		log.Error("Invalid Msg Type: ", msgType)
		return
	}

	// 将数据编码为字符串
	data, err := json.Marshal(postData)

	// 创建HTTP客户端
	client := &http.Client{}

	// 创建请求
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(data))
	if err != nil {
		log.Error("http.NewRequest error:", err)
		return
	}

	// 设置请求头，指定内容类型和长度
	req.Header.Set("Content-Type", "application/json")

	// 发送请求
	resp, err := client.Do(req)
	if err != nil {
		log.Error("http.NewRequest do error:", err)
		return
	}

	defer resp.Body.Close()

	bodyResp, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Error("http.NewRequest io.ReadAll error:", err)
		return
	}

	log.Debug(string(bodyResp))
}
