package main

import (
	"fmt"
	"net/http"
  "strconv"
	"github.com/gorilla/mux"
  "io/ioutil"
  "strings"
  "time"
  "os"
  "github.com/aquilax/tripcode"
  "regexp"
  "crypto/sha1"
  "encoding/hex"
  //"html"
  "github.com/microcosm-cc/bluemonday"
  //"gopkg.in/russross/blackfriday.v2"
  "sort"
)

type Post struct {
  id int64
  thread int64
  comment string
  name string
  trip string
  formatting string
  ip string
  when time.Time
  password string
  deleted bool
  subject string
}

const admin_op_only bool = true
const sort_bump_time = false
const site_name = "Ue Kuniyoshi's and Ion Müller's Web Site"

func stringInSlice(a string, list []string) bool {
    for _, b := range list {
        if b == a {
            return true
        }
    }
    return false
}

func postBox(id int64) string {
  subjectin := ""
  if id == 0 {
    subjectin = "<tr><td>Subject</td><td><input type=\"text\" name=\"subject\"></td></tr>"
  }
  return `<form action="/post" method="post">
<input type="hidden" name="thread" value="`+strconv.FormatInt(id,10)+`">
<table>
<tr><td>Name</td><td><input type="text" name="name"> <input type="submit" value="Post"></td></tr>
`+subjectin+`
<tr><td>Comment</td><td><textarea name="comment"></textarea></td></tr>
<tr><td>Formatting</td><td><select name="formatting"><option value="html">HTML</option></select></td></tr>
<tr><td>Password</td><td><input type="password" name="password"></td></tr></table></form>`
}

func writePost(p Post) error {
  r, err := os.OpenFile(fmt.Sprintf("threads/%d.thread", p.thread), os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0644) 

  h := sha1.New()
  h.Write([]byte(p.password))
  _, err = r.WriteString(fmt.Sprintf("%s\x1f%s\x1f%s\x1f%s\x1f%s\x1f%s\x1f%s\x1f%t\x1f%s\x1e", p.comment, p.name, p.trip, p.formatting, p.ip, p.when.Format(time.RFC3339), hex.EncodeToString(h.Sum(nil)), p.deleted, p.subject))

  if err != nil {
    return err
  }
  
  r.Close()
  return nil
}

func deletePost(p Post) error {
    ap, err := readThread(p.thread)
  if err != nil {
    return err
  }

  
  f, err := os.Create(fmt.Sprintf("threads/%d.thread", p.thread))
  if err != nil {
    return err
  }
  f.Close()
  
  for _,v := range ap {
    fmt.Println("pp: %+v\n", v)
    if v.id == p.id {
      v.deleted = true
    }
    err = writePost(v)
    if err != nil {
      return err
    }
  }
  return nil
}

func readThread(tid int64) ([]Post, error) {
  c, err := ioutil.ReadFile(fmt.Sprintf("threads/%d.thread", tid))
  if err != nil {
    return nil, err
  }
  posts := strings.Split(string(c), "\x1e")
  var pret []Post
  for i, p := range posts {
    attrs := strings.Split(p, "\x1f")
    if len(attrs) == 9 {
      fmt.Printf("%d\n", len(attrs))
      t1, e := time.Parse(time.RFC3339, attrs[5])
      if e != nil {
        return nil, e
      }
      b := false
      if attrs[7] == "true" {
        b = true
      }
      pret = append(pret, Post {int64(i+1), tid, attrs[0], attrs[1], attrs[2], attrs[3], attrs[4], t1, attrs[6], b, attrs[8]})
    }
  }
  return pret, nil
}

func filterPosts(filter string, posts []Post) ([]Post, string) {
  var plist []Post
  fr, err := regexp.Compile(`((((l|f)?\d+|(\d+\-(\d+)?))(,|$))+)`)
  if err == nil {
    if fr.Find([]byte(filter)) != nil {
      selections := strings.Split(filter, ",")
      for _, selection := range selections {
        lower_bound := int64(0)
        upper_bound := int64(0)
        
        rangereg, _ := regexp.Compile(`(\d+)-(\d+)?`)
        if rangereg.FindString(selection) != "" {
          if rangereg.FindAllStringSubmatch(selection, 2)[0][2] != "" {
            lboundstr := rangereg.FindAllStringSubmatch(selection, 2)[0][1]
            uboundstr := rangereg.FindAllStringSubmatch(selection, 2)[0][2]
            lower_bound, err = strconv.ParseInt(lboundstr, 10, 64)
            if err != nil {
              return nil, fmt.Sprintf("Error parsing lower bound of range: %s", lboundstr)
            }
            upper_bound, err = strconv.ParseInt(uboundstr, 10, 64)
            if err != nil {
              return nil, fmt.Sprintf("Error parsing upper bound of range: %s", uboundstr)
            }
          } else {
            lboundstr := rangereg.FindAllStringSubmatch(selection, 2)[0][1]
            lower_bound, err = strconv.ParseInt(lboundstr, 10, 64)
            if err != nil {
              return nil, fmt.Sprintf("Error parsing lower bound of range: %s", lboundstr)
            }
            upper_bound = int64(len(posts))
          }
        } else { 
          flreg, _ := regexp.Compile(`(f|l)?(\d+)-?$`)
          if flreg.FindString(selection) != "" {
            if flreg.FindAllStringSubmatch(selection, 2)[0][1] != "" {
              control := flreg.FindAllStringSubmatch(selection, 2)[0][1]
              numstr := flreg.FindAllStringSubmatch(selection, 2)[0][2]
              num, err := strconv.ParseInt(numstr, 10, 64)
              if err != nil {
                return nil, fmt.Sprintf("Error parsing value of f/l: %s", numstr)
              }
              if control == "l" {
                lower_bound = int64(len(posts))-num
                upper_bound = int64(len(posts))
              } else {
                lower_bound = 1
                upper_bound = num
              }
            } else {
              numstr := flreg.FindAllStringSubmatch(selection, 2)[0][2]
              num, err := strconv.ParseInt(numstr, 10, 64)
              if err != nil {
                return nil, fmt.Sprintf("Error parsing value of number: %s", numstr)
              }
              lower_bound = num
              upper_bound = num
            }
          }
        }

        if lower_bound < 1 {
          lower_bound = 1
        }

        if upper_bound > int64(len(posts)) {
          upper_bound = int64(len(posts))
        }
      
        if lower_bound < 1 || upper_bound < 1 || lower_bound > upper_bound || upper_bound > int64(len(posts)) {
          return nil, fmt.Sprintf("Bounds out of range: %d - %d", lower_bound, upper_bound)
        }
        //http.Error(w, fmt.Sprintf("Bounds: %d - %d\n", lower_bound, upper_bound), http.StatusForbidden)
        for i := lower_bound-1; i < upper_bound; i++ {
          plist = append(plist, posts[i])
        }
      }
    } else {
      for i,e := range posts {
        e.id = int64(i+1)
        plist = append(plist, e)
      }
    }
  }
  return plist, ""
}

func maxThreadId() (int64, error) {
  files, err := ioutil.ReadDir("threads")
	if err != nil {
		return 0, err
	}

  var maxid int64 = 0
	for _, file := range files {
		fmt.Println(file.Name())
    s := strings.TrimSuffix(file.Name(), ".thread")
    tid, _ := strconv.ParseInt(s, 10, 32)
    // if err != nil {
    //   return 0, err
    // }
    if tid > maxid {
      maxid = tid
    }
	}
  return maxid, nil
}

func allPosts() ([][]Post, error) {
  files, err := ioutil.ReadDir("threads")
	if err != nil {
		return nil, err
	}

  var idli []int64
  var ops [][]Post
	for _, file := range files {
		fmt.Println(file.Name())
    s := strings.TrimSuffix(file.Name(), ".thread")
    tid, _ := strconv.ParseInt(s, 10, 32)
    idli = append(idli, tid)
  }
  for _,id := range idli {
    ps, err := readThread(id)
    if err != nil {
      return nil, err
    }
    ops = append(ops, ps)
  }
  if(sort_bump_time) {
    sort.Slice(ops[:], func(i, j int) bool {
      return ops[i][len(ops[i])-1].when.After(ops[j][len(ops[j])-1].when)
    })
  } else {
    sort.Slice(ops[:], func(i, j int) bool {
      return ops[i][0].when.After(ops[j][0].when)
    })
  }
  return ops, nil
}

func formatThread(thread []Post, abbrev bool) string {
  s := "<div class=\"thread\"><div class=\"inner-thread\">"
  s = s+"<h2 class=\"thread-title\"><a href=\"/thread?id="+strconv.FormatInt(thread[0].thread, 10)+"\">"+thread[0].subject+"</a></h2>"
  if !abbrev {
    s += "<p>Brought to you by <b>"+site_name+"</b>.</p>"
  }
  s += formatPost(thread[0])
  if len(thread) > 1 {
    var sel []Post
    sel = thread[1:]
    if abbrev && len(thread) > 5 {
      sel = thread[len(thread)-1-5:len(thread)-1]
    }
    for _,v := range sel {
      s += formatPost(v)
    }
  }
  if abbrev {
    s += postBox(thread[0].thread)
  }
  s += "</div></div>"
  return s
}

func formatPage(content string, indexpage bool) string {
  c := "threadpage"
  if indexpage {
    c = "indexpage"
  }
  return `<!doctype html><html><head>
<style type="text/css">
html { padding:0px; margin:0px; }
body { padding:8px; margin:0px; color:black; background-image: url(data:image/gif;base64,R0lGODlhPAA8AJEAANzAptCznMWtmbOekiH5BAQUAP8ALAAAAAA8ADwAAAL/nI+py+0YnpzUCRGyDnjzC4bipXWeBgBnKaRuNgpDmKll4Oa6jvN1ihNFLCJEa4WDeWIjgOmzAX1InOGhukxJe5kBVChLopwmgPcyw8hmwC11YBsOhVjOK5neeGGdq9ZUBJUWRhU11uLEpsfSZdBDwjQV0RFWc6Li1dZyoVVlcoCoBsk0ShN1RPby9KVm9SgVGanEh2HzstfDVnhwBznDMUi6Uhu34VLowQtUEmww8ru62pMCZ+tsA+ioCUJWSdo6RaxyV6PtXOVHJgkCFz6yVOWUM0VtkEir7XQjdYYmSzvtxzhsJKAByxSnVIRJ336FG/hD3CJP9pbBYgcwRBpc/8bI2fnYQR+MitigYaSlUdoYS4XaRNmIUBK+byqn7ZhHjGWimGocQqlzCUU5aM7QgHpwZFAwGUybzqgAVcGfNVEdzBuzbEUORDyW3bxpLMw8dYUIcc0qRoOjjh62TkszsN6sUn/qFJMzrFJatSSAYOJ44giXjoiA0nGXsnCudjfi3NWK5CLDZzX6TrrbCahWGoLfIPb1g48ueEhuoET2E6XS0GYuG3pUOrLk0884qZBhpsVGSj4js4C0RBihNl5MBos9CyUjd3xEc9bNeJClzcOWewPjrxbL6DWnnxL1ZMvvIi0bkTkOA+3E5r+/u7myBI4/rYBFav4YWKZRwSy6tf/CtpByyVUXGBUh2FKCDbjcZl9bsQECj4ELCoVbcv6FJgZ/SliECxP2CERIWjsIdFc0irlyHzpwLHDQUjRld04RCVC1AI0yOqPAi+LpiBpJX/2oQ0hADvlVcLPF0EiAnTTk0EXasHPbczEopUw3/Xii1h7NfJhHhsC5xw58UUrzSVl9YMARIAYCFA4CW+kVTXIpSacmKS5SNc1zknQZJlPSJQTeB/34h6NHA7525hrL7cPORYEg9JiRkdhzWHs0sYHQgSM9qEc/RmX0DhOtbZMlWYFg1Od4l9KQaYqiLVrKOaoiOVlxobAXHG1uMHXKZ3aKFWmuBpYSqodTNlqNJgUpircnsWBmd6qfXMCC3EiiZASqKVMWNxhyJuKaWh1SOsqterI5apkUBQAAOw==); }

body.threadpage {
background-color:#efefef;
	background-image:none;
}
.threadpage .thread {
	margin:0;
	background:none;
	padding:0;
	border:0;
}
.threadpage .inner-thread {
	border:0;
	margin:0;
}

.thread {
margin-left: 2.5%;
	margin-right: 2.5%;
	position: relative;
	background: #efefef;
	border: 1px outset white;
	padding: 7px;
	clear: both;
margin-bottom:1em;
}
.title-box {
background-color:#ccffcc;
}
.title-box h1 {
font-size:1.5em;
font-weight:normal;
padding:0;
margin:0;
}
.inner-thread {
margin:1px;
border:1px solid #afafaf;
padding:0 0.3em;
}
.text {
margin:0.5em 0em 0em 1.4em;
}
.post {
margin: 0px 0px 0.5em 0px;
}
.infoline .name {
color:green;
}
.thread-title a {
color:#ff0000;
text-decoration:none;
}
.thread-title {
font-weight:bold;
margin-bottom:0.4.em;

}
.threadpage .thread-title {
font-weight:normal;
margin-top:0;
}
</style></head><body class="`+c+`">`+content+"</body></html>"
}

func formatPost(p Post) string {
  t := ""
  if len(p.trip) > 0 {
    t = "!"+p.trip
  }

  //p.comment = html.EscapeString(p.comment)
  
  //unsafe := blackfriday.Run([]byte(p.comment))
  p.comment = string(bluemonday.UGCPolicy().SanitizeBytes([]byte(p.comment)))
  r, _ := regexp.Compile(`&gt;&gt;((((l|f)?\d+|(\d+\-(\d+)?))(,|$))+)`)
  p.comment = string(r.ReplaceAll([]byte(p.comment), []byte("<a href=\"/thread?id="+strconv.FormatInt(p.thread, 10)+"&p=$1\">&gt;&gt;$1</a>")))
  p.comment = strings.Replace(p.comment, "\n", "<br>", -1)
  if p.deleted {
    p.comment = "deleted"
  }
  return fmt.Sprintf("<div class=\"post\"><div class=\"infoline\"><b>%d</b> &nbsp;Name: <span class=\"name\"><b>%s</b><span class=\"trip\">%s</span></span> : %s</div><div class=\"text\">%s</div></div>", p.id,
    p.name, t, p.when.Format("2006-01-02 15:04"), p.comment)
}

func main() {
  var admin_trips = []string {"ue!7olmB2ux1w", "ion!9SzFs8qkPA"}
	r := mux.NewRouter()

  r.HandleFunc("/del", func(w http.ResponseWriter, r *http.Request) {
    tid, err := strconv.ParseInt(r.FormValue("id"), 10, 32)
    if err != nil {
      http.Error(w, "Could not undersatnd that thread id.", http.StatusForbidden)
      return
    }
    posts, err := readThread(tid)
    var plist []Post
    if err != nil {
      http.Error(w, "Error reading thread.", http.StatusForbidden)
      return
    }

    plist, errstr := filterPosts(r.FormValue("p"), posts)
    if plist == nil {
      http.Error(w, errstr, http.StatusForbidden)
      return
    }

    if len(r.FormValue("password")) == 0 {
      http.Error(w, "Enter a password to delete posts.", http.StatusForbidden)
        return
    }

    for _,p := range plist {
      if p.password == "" {
        http.Error(w, fmt.Sprintf("Post %d has no password so it can't be deleted.", p.id), http.StatusForbidden)
        return
      }
      h := sha1.New()
      h.Write([]byte(r.FormValue("password")))
      if hex.EncodeToString(h.Sum(nil)) == p.password {
        err = deletePost(p)
        if err != nil {
          http.Error(w, "Could not delete a post", http.StatusForbidden)
          return
        }
      } else {
        http.Error(w, "Incorrect password", http.StatusForbidden)
        return
      }
    }
    fmt.Fprintf(w, "Post(s) deleted.\n")
  })
  
	r.HandleFunc("/thread", func(w http.ResponseWriter, r *http.Request) {
    tid, err := strconv.ParseInt(r.FormValue("id"), 10, 32)
    if err != nil {
      http.Error(w, "Could not undersatnd that thread id.", http.StatusForbidden)
      return
    }
    posts, err := readThread(tid)
    var plist []Post
    if err != nil {
      http.Error(w, "Error reading thread.", http.StatusForbidden)
      return
    }

    plist, errstr := filterPosts(r.FormValue("p"), posts)
    if plist == nil {
      http.Error(w, errstr, http.StatusForbidden)
      return
    }
    
    
    s := formatThread(plist, false)
    s += "<hr><a href=\"/\">Return</a> <a href=\"/thread?id="+r.FormValue("id")+"\">Entire thread</a> <a href=\"/thread?id="+r.FormValue("id")+"&p=l50\">Last 50 posts</a>"
    s += "<h3>New reply</h3>"+postBox(tid)
    s += "<hr><form action=\"/del\" method=\"post\"><input type=\"hidden\" name=\"id\" value=\""+r.FormValue("id")+"\">Delete posts: <input type=\"text\" name=\"p\"> with password: <input type=\"password\" name=\"password\"> <input type=\"submit\" value=\"Delete post(s)\"></form>"
    fmt.Fprint(w, formatPage(s, false))
  })
    
	r.HandleFunc("/post", func(w http.ResponseWriter, r *http.Request) {
    tid, err := strconv.ParseInt(r.FormValue("thread"), 10, 32)
    if err != nil {
      http.Error(w, "my own error message", http.StatusForbidden)
      return
    }

    name := r.FormValue("name")
    trip := ""
    if len(r.FormValue("name")) == 0 {
      name = "Anonymous"
    } else {
      cmp := strings.Split(r.FormValue("name"), "!")
      if len(cmp) > 1 {
        name = cmp[0]
        trip = tripcode.Tripcode(strings.Join(cmp[1:], "!"))
      }
    }

    if tid == 0 && admin_op_only && !stringInSlice(name+"!"+trip, admin_trips) {
      http.Error(w, "Only admins may make new threads.", http.StatusForbidden)
      return
    }
    
    if len(r.FormValue("comment")) == 0 {
      http.Error(w, "You must enter a comment.", http.StatusForbidden)
      return
    }
    if len(r.FormValue("formatting")) == 0 {
      http.Error(w, "You must enter a formatting option.", http.StatusForbidden)
      return
    }
    if len(r.FormValue("subject")) == 0 && tid == 0 {
      http.Error(w, "You must enter a subject value.", http.StatusForbidden)
      return
    }

    if strings.Contains(r.FormValue("comment"), "\x1f") ||
      strings.Contains(r.FormValue("comment"), "\x1e") ||
      strings.Contains(r.FormValue("name"), "\n") ||
      strings.Contains(r.FormValue("subject"), "\n"){
      http.Error(w, "Invalid characters found in post information.", http.StatusForbidden)
      return
    }
    
    fmt.Fprintf(w, "your thread id: %d\n", tid)

    subj := r.FormValue("subject")
    if tid > 0 {
      posts, err := readThread(tid)
      if err != nil {
        http.Error(w, "Could not read thread.", http.StatusForbidden)
        return
      }
      subj = posts[0].subject
    }

    if tid == 0 {
      x, err := maxThreadId()
      if err != nil {
        http.Error(w, "Couldn't create thread: couldn't get a new thread id", http.StatusForbidden)
        return
      }
      tid = x+1
    }
    
    writePost(Post{0, tid, r.FormValue("comment"), name, trip, r.FormValue("formatting"), r.RemoteAddr, time.Now(), r.FormValue("password"), false, subj})
	})
  
  r.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
    plist, err := allPosts()
    if err != nil {
      http.Error(w, "Unable to get list of all the threads.", http.StatusForbidden)
    }
    s :=`<div class="thread title-box"><div class="inner-thread"><h1>`+site_name+`</h1></div></div>`
    //fmt.Fprintf(w, "%v\n", plist)
    for _,thread := range plist {
      s += formatThread(thread, true)
    }
    s += "<div class=\"thread title-box\"><div class=\"inner-thread\"><h3>New thread</h3>"+postBox(0)+"</div></div></body></html>"
    fmt.Fprint(w, formatPage(s, true))
	})

	http.ListenAndServe(":80", r)
}
