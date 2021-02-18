package main

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path"
	"regexp"
	"strconv"
	"sync"

	"github.com/axgle/mahonia"
)

var (
	//<div class="classify clearfix"> </div>
	regUrlDiv = `<div class="classify clearfix">(.*?)</div>`

	//<a href="/4kfengjing/" title="4K风景图片">4K风景</a>
	//regUrl = `<a href="(.*?)"[\s\S]+?>(.*?)</a>`
	regUrl = `<a href="(/.*?/)"[\s\S]+?>(.*?)</a>`

	//<img src="/uploads/allimg/180826/113958-1535254798fc1c.jpg" alt="阿尔卑斯山风景4k高清壁纸3840x2160">
	//<img src="/uploads/allimg/200312/235114-1584028274e442.jpg" alt="陆萱萱 黑色丝袜美腿 养眼好看身材4k美女壁纸" />
	regImage = `<img src="(/.*?)" alt="(.*?)"[\s\S]*?>`

	regH1 = `<h1[\s\S]*?>(.*?)</h1>`
	//<a href="/4kzongjiao/index_5.html">5</a><a href="/4kzongjiao/index_2.html">下一页</a>
	regPage = `<a href="[\w./]+?">([\d]{1,3})</a><a href="[\w./]+?">下一页</a>`

	imagesChan      = make(chan int, 5) //阻塞并发5执行
	imagesGroupChan = make(chan map[string]string, 1000)
	downwait        sync.WaitGroup
	urlwait         sync.WaitGroup
)

func getInfo(url string) (resHTML string, err error) {
	client := &http.Client{}
	req, _ := http.NewRequest("GET", url, nil)
	// 自定义Header
	req.Header.Set("User-Agent", "Mozilla/4.0 (compatible; MSIE 6.0; Windows NT 5.1)")
	resp, err1 := client.Do(req)
	if err1 != nil {
		err = err1
		return
	}
	defer resp.Body.Close()

	// p, err2 := goquery.NewDocumentFromResponse(resp)
	// if err2 != nil {
	// 	err = err2
	// 	return
	// }
	//doc = p
	//fmt.Println(doc.Html())

	body, _ := ioutil.ReadAll(resp.Body)
	resHTML = mahonia.NewDecoder("gbk").ConvertString(string(body)) //gbk=>utf8
	return
}

//判断文件文件夹是否存在
func createDateDir(folderPath string) {
	if _, err := os.Stat(folderPath); os.IsNotExist(err) {

		// 必须分成两步
		// 先创建文件夹
		os.MkdirAll(folderPath, 0777)
		//fmt.Println(err1)
		// 再修改权限
		os.Chmod(folderPath, 0777)
	}
}

func getURLs(url string) (res []string) {
	res = make([]string, 0)
	resp, err := getInfo("http://pic.netbian.com/")
	if err != nil {
		fmt.Println(err)
		return
	}
	re := regexp.MustCompile(regUrl)
	urlsDiv := re.FindAllStringSubmatch(resp, -1)

	//tmpHTML := urlsDiv[0][0]

	// aresp := regexp.MustCompile(regUrl)
	// aInfos := aresp.FindAllStringSubmatch(tmpHTML, -1)

	for _, aInfo := range urlsDiv {
		//fmt.Println(i, "-----", aInfo[1])
		url = "http://pic.netbian.com" + aInfo[1]
		res = append(res, url)
	}
	return
}

func getPageInfo(url string) []map[string]string {
	images := make([]map[string]string, 0)
	resp, err := getInfo(url)
	if err != nil {
		fmt.Println(err)
		return images
	}

	rh1 := regexp.MustCompile(regH1)
	r := rh1.FindStringSubmatch(resp)

	re := regexp.MustCompile(regImage)
	rets := re.FindAllStringSubmatch(resp, -1)
	for _, ret := range rets {
		imageInfo := make(map[string]string)
		imageInfo["imgUrl"] = "http://pic.netbian.com" + ret[1]
		imageInfo["alt"] = ret[2]
		imageInfo["menu"] = r[1]
		//fmt.Println(imageInfo)
		images = append(images, imageInfo)
	}
	return images
}

func downloadImage(url, filename, pathUrl string) {
	client := &http.Client{}
	req, _ := http.NewRequest("GET", url, nil)
	// 自定义Header
	req.Header.Set("User-Agent", "Mozilla/4.0 (compatible; MSIE 6.0; Windows NT 5.1)")
	resp, err1 := client.Do(req)
	if err1 != nil {
		fmt.Println(err1)
		return
	}
	defer resp.Body.Close()

	imageBytes, _ := ioutil.ReadAll(resp.Body)
	suffix := path.Ext(url)

	//判断文件目录是否存在
	createDateDir(pathUrl)

	filepath := pathUrl + filename + suffix
	err2 := ioutil.WriteFile(filepath, imageBytes, 0644)
	if err2 != nil {
		fmt.Println(filename, "---", url, "---", "下载失败:", err2)
		return
	}
	fmt.Println(filename, "---", url, "---", "下载成功")
}

func startDown(url string) {
	images := getPageInfo(url)
	fmt.Println("--------", len(images), "条数据--------")
	for _, image := range images {
		//downloadImage(image["imgUrl"], image["alt"], dir)
		//fmt.Println(image["alt"], "--------end")
		imagesGroupChan <- image
	}
}

func getPageCount(url string) int {
	html, err := getInfo(url)
	if err != nil {
		fmt.Println(err)
	}
	res := regexp.MustCompile(regPage)
	ret := res.FindStringSubmatch(html)
	//fmt.Println(ret[1])
	page, _ := strconv.Atoi(ret[1])
	return page
}

func pageTask(url string) {

	start := 1
	end := getPageCount(url)
	//end := 3

	if end <= start {
		fmt.Println("page error")
		return
	}

	urlnew := url
	for i := start; i <= end; i++ {
		if i > 1 {
			urlnew = url + "index_" + strconv.Itoa(i) + ".html"
		}
		urlwait.Add(1)
		go func(pageURL string) {
			startDown(pageURL)
			urlwait.Done()
		}(urlnew)
	}
}

func main() {
	imgURLs := getURLs("http://pic.netbian.com/")
	for _, url := range imgURLs {
		fmt.Println(url)
		pageTask(url)
	}

	// url := "http://pic.netbian.com/uploads/allimg/210111/205558-16103697588a25.jpg"
	// dir, _ := os.Getwd()
	// dir = dir + "/images/美女壁纸/"
	// alt := "text"
	// downloadImage(url, alt, dir)

	//url := "http://pic.netbian.com/4kzongjiao/"

	go func() {
		urlwait.Wait()
		close(imagesGroupChan)
	}()

	for imageGroup := range imagesGroupChan {
		downwait.Add(1)
		go func(image map[string]string) {
			imagesChan <- 1
			dir, _ := os.Getwd()
			dir = dir + "/images/" + image["menu"] + "/"
			downloadImage(image["imgUrl"], image["alt"], dir)
			<-imagesChan
			downwait.Done()
		}(imageGroup)
		//fmt.Println(imageGroup)
	}
	downwait.Wait()
}
