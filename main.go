package main

import (
	"flag"
	"fmt"
	"livereload/colorlog"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/golang/glog"
)

var watchExts = []string{".go"}

var (
	eventTime    = make(map[string]int64)
	scheduleTime time.Time
)

const usage = `

 Usage:

  	-h    显示当前帮助信息；
  	-f	  指定main文件；
  	-o    执行编译后的可执行文件名；
  	-r    是否搜索子目录，默认为true；
`

type watch struct {

	//热编译相关
	appName   string    // 输出的程序文件
	appCmd    *exec.Cmd // appName的命令行包装引用，方便结束其进程。
	goCmdArgs []string  // 传递给go build的参数
}

func ColorLog(format string, a ...interface{}) {
	logStr := colorlog.ColorLogS(format, a...)
	glog.InfoDepth(1, logStr)
}

var wathDir = flag.String("watch", "", "watch dir")

// TODO: add watch dir
func main() {

	// 初始化flag
	var showHelp, recursive bool
	var outputName, mainFiles string

	flag.BoolVar(&showHelp, "h", false, "显示帮助信息")
	flag.BoolVar(&recursive, "r", true, "是否查找子目录")
	flag.StringVar(&outputName, "o", "", "指定输出名称")
	flag.StringVar(&mainFiles, "f", "", "指定需要编译的文件")
	flag.Usage = func() {
		fmt.Println(usage)
	}

	flag.Lookup("logtostderr").Value.Set("true")

	flag.Parse()

	if showHelp {
		flag.Usage()
		return
	}

	wd, err := os.Getwd()
	if err != nil {
		ColorLog("[ERRO] 获取当前工作目录时，发生错误: [ %s ]", err)
		return
	}

	if *wathDir != "" {
		err := os.Chdir(*wathDir)
		if err != nil {
			ColorLog("[ERROR] failed to move into folder %s", *wathDir)
		}
	}

	// 初始化goCmd的参数
	args := []string{"build", "-o", outputName}
	if len(mainFiles) > 0 {
		args = append(args, mainFiles)
	}

	w := &watch{
		appName:   getAppName(outputName, wd),
		goCmdArgs: args,
	}

	w.watcher(recursivePath(recursive, append(flag.Args(), wd)))

	go w.build()

	done := make(chan bool)
	<-done
}

func (w *watch) watcher(paths []string) {

	ColorLog("[TRAC] wather begin...")
	//初始化监听器
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		ColorLog("[ERRO] 初始化监视器失败: [ %s ]", err)
		os.Exit(2)
	}

	go func() {
		for {
			select {
			case event := <-watcher.Events:
				build := true
				if !w.checkIfWatchExt(event.Name) {
					continue
				}
				ColorLog("[TRAC] %s file %s", event.Op.String(), event.Name)
				if event.Op&event.Op == fsnotify.Chmod {
					// if event.Op&event.Chmod == fsnotify.Chmod {
					ColorLog("[SKIP] [ %s ]", event)
					continue
				}

				mt := w.getFileModTime(event.Name)
				if t := eventTime[event.Name]; mt == t {
					// ColorLog("[SKIP] [ %s ]", event.String())
					build = false
				}

				eventTime[event.Name] = mt

				if build {
					go func() {
						time.Sleep(time.Microsecond * 200)
						// ColorLog("[TRAC] 触发编译事件: < %s >", event)
						w.build()
					}()
				}

			case err := <-watcher.Errors:
				ColorLog("[ERRO] 监控失败 [ %s ]", err)
			}
		}
	}()

	for _, path := range paths {
		// ColorLog("[TRAC] 监视文件夹: ( %s )", path)
		err = watcher.Add(path)
		if err != nil {
			ColorLog("[ERRO] 监视文件夹失败: [ %s ]", err)
			os.Exit(2)
		}
	}
}

// 开始编译代码
func (w *watch) build() {
	ColorLog("[INFO] build < %s >...", w.appName)

	goCmd := exec.Command("go", w.goCmdArgs...)
	goCmd.Stderr = os.Stderr
	goCmd.Stdout = os.Stdout

	if err := goCmd.Run(); err != nil {
		ColorLog("[ERRO] 编译失败: [ %s ]", err)
		return
	}

	ColorLog("[SUCC] build < %s > success !", w.appName)

	w.restart()
}

func (w *watch) restart() {
	defer func() {
		if err := recover(); err != nil {
			fmt.Println("Kill.recover -> ", err)
		}
	}()

	if w.appCmd != nil && w.appCmd.Process != nil {
		// ColorLog("[INFO] 终止旧进程...")
		if err := w.appCmd.Process.Kill(); err != nil {
			ColorLog("[ERROR] 终止进程失败 [ %s ] ...\n", err)
		}
		// ColorLog("[SUCC] 旧进程被终止! ")
	}

	// ColorLog("[INFO] restart < %s >", w.appName)
	if strings.Index(w.appName, "./") == -1 {
		w.appName = "./" + w.appName
	}

	// ColorLog("[INFO] 启动新进程... \n")
	w.appCmd = exec.Command(w.appName)
	w.appCmd.Stderr = os.Stderr
	w.appCmd.Stdout = os.Stdout
	if err := w.appCmd.Start(); err != nil {
		// ColorLog("[ERRO] 启动进程时出错: [ %s ] \n", err)
	}

	ColorLog("[SUCC] new < %s > restarted !", w.appName)
}

func (w *watch) checkIfWatchExt(name string) bool {
	for _, s := range watchExts {
		if strings.HasSuffix(name, s) {
			return true
		}
	}
	return false
}

func (w *watch) getFileModTime(path string) int64 {
	path = strings.Replace(path, "\\", "/", -1)
	f, err := os.Open(path)
	if err != nil {

		ColorLog("[ERRO] 文件打开失败 [ %s ]", err)
		return time.Now().Unix()
	}
	defer f.Close()

	fi, err := f.Stat()
	if err != nil {
		ColorLog("[ERRO] 获取不到文件信息 [ %s ]", err)
		return time.Now().Unix()
	}

	return fi.ModTime().Unix()
}

func getAppName(outputName, wd string) string {
	if len(outputName) == 0 {
		outputName = filepath.Base(wd)
	}
	if runtime.GOOS == "windows" && !strings.HasSuffix(outputName, ".exe") {
		outputName += ".exe"
	}
	if strings.IndexByte(outputName, '/') < 0 || strings.IndexByte(outputName, filepath.Separator) < 0 {
		outputName = outputName
	}

	return outputName
}

// 根据recursive值确定是否递归查找paths每个目录下的子目录。
func recursivePath(recursive bool, paths []string) []string {
	if !recursive {
		return paths
	}

	ret := []string{}

	walk := func(path string, fi os.FileInfo, err error) error {
		if err != nil {
			ColorLog("[ERRO] 遍历监视目录错误: [ %s ]", err)
		}

		//(BUG):不能监视隐藏目录下的文件
		if fi.IsDir() && strings.Index(path, "/.") < 0 {
			ret = append(ret, path)
		}
		return nil
	}

	for _, path := range paths {
		if err := filepath.Walk(path, walk); err != nil {
			ColorLog("[ERRO] 遍历监视目录错误: [ %s ]", err)
		}
	}

	return ret
}
