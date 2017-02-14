package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/mozhata/livereload/colorlog"
)

const usage = `

 Usage:

  	-h     show help informations;
  	-f	   the main file;
  	-o     the ouput binary name;
  	-r     watch recursively; default true;
  	-watch which folder shold watch;
`

var (
	showHelp   = flag.Bool("h", false, "show help message")
	recursive  = flag.Bool("r", true, "watch recursively")
	wathDir    = flag.String("watch", "", "which folder should watch")
	outputName = flag.String("o", "", "the binary name")
	mainFiles  = flag.String("f", "", "the main.go file")

	eventTime = make(map[string]int64)

	watchExts = []string{".go"}

	logger = colorlog.NewLogger("livereload: ")
)

//热编译相关
type watch struct {
	appName   string    // 输出的程序文件
	appCmd    *exec.Cmd // appName的命令行包装引用，方便结束其进程。
	goCmdArgs []string  // 传递给go build的参数
}

func main() {
	flag.Usage = func() {
		fmt.Println(usage)
	}
	flag.Parse()

	if *showHelp {
		flag.Usage()
		return
	}

	wd, err := os.Getwd()
	if err != nil {
		logger.Error("can't get the current folder: [%s]", err)
		return
	}

	if *wathDir != "" {
		err := os.Chdir(*wathDir)
		if err != nil {
			logger.Error("failed to move into folder %s: [%s]", *wathDir, err)
		}
	}

	// 初始化goCmd的参数
	args := []string{"build", "-o", *outputName}
	if len(*mainFiles) > 0 {
		args = append(args, *mainFiles)
	}

	w := &watch{
		appName:   getAppName(*outputName, wd),
		goCmdArgs: args,
	}

	w.watcher(recursivePath(*recursive, append(flag.Args(), wd)))

	go w.build()

	done := make(chan bool)
	<-done
}

func (w *watch) watcher(paths []string) {

	logger.Trace("watcher begin...")
	//初始化监听器
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		logger.Error("failed to watch the folder: [%s]", err)
		os.Exit(2)
	}

	go func() {
		for {
			select {
			case event := <-watcher.Events:
				if event.Op == fsnotify.Create {
					finfo, err := os.Stat(event.Name)
					if err != nil {
						logger.Error("os.Stat(%s) err. err: %s", event.Name, err)
					}
					if finfo.IsDir() {
						logger.Info("add new floder %s to watcher", event.Name)
						watcher.Add(event.Name)
						continue
					}
				}
				build := true
				if !w.checkIfWatchExt(event.Name) {
					continue
				}
				logger.Trace("%s file %s", event.Op.String(), event.Name)
				if event.Op&event.Op == fsnotify.Chmod {
					logger.Warning("SKIP [%s]", event)
					continue
				}

				// mt := w.getFileModTime(event.Name)
				// if t := eventTime[event.Name]; mt == t {
				// 	build = false
				// }

				// eventTime[event.Name] = mt

				if build {
					go func() {
						time.Sleep(time.Microsecond * 200)
						w.build()
					}()
				}

			case err := <-watcher.Errors:
				logger.Error("watch folder err: [%s]", err)
			}
		}
	}()

	for _, path := range paths {
		err = watcher.Add(path)
		if err != nil {
			logger.Error("watch folder err [%s]", err)
			os.Exit(2)
		}
	}
}

// 开始编译代码
func (w *watch) build() {
	logger.Info("build < %s >...", w.appName)

	goCmd := exec.Command("go", w.goCmdArgs...)
	goCmd.Stderr = os.Stderr
	goCmd.Stdout = os.Stdout

	if err := goCmd.Run(); err != nil {
		logger.Error("build faild: [%s]", err)
		return
	}

	logger.Success("build < %s > success !", w.appName)

	w.restart()
}

func (w *watch) restart() {
	defer func() {
		if err := recover(); err != nil {
			fmt.Println("Kill.recover -> ", err)
		}
	}()

	if w.appCmd != nil && w.appCmd.Process != nil {
		if err := w.appCmd.Process.Kill(); err != nil {
			logger.Error("failed to kill precess [%s]", err)
		}
	}

	if strings.Index(w.appName, "./") == -1 {
		w.appName = "./" + w.appName
	}

	w.appCmd = exec.Command(w.appName)
	w.appCmd.Stderr = os.Stderr
	w.appCmd.Stdout = os.Stdout
	if err := w.appCmd.Start(); err != nil {
	}

	logger.Success("new < %s > restarted !", w.appName)
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

		logger.Error("failed to open file [%s]", err)
		return time.Now().Unix()
	}
	defer f.Close()

	fi, err := f.Stat()
	if err != nil {
		logger.Error("can not find file infos [%s]", err)
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
			logger.Error("walkDir error [%s]", err)
		}

		//(BUG):不能监视隐藏目录下的文件
		if fi.IsDir() && strings.Index(path, "/.") < 0 {
			ret = append(ret, path)
		}
		return nil
	}

	for _, path := range paths {
		if err := filepath.Walk(path, walk); err != nil {
			logger.Error("walk dir err [%s]", err)
		}
	}

	return ret
}
