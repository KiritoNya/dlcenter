var url = "http://localhost:8090/show"
var container = document.getElementById("container")
var count=1

window.setInterval(function GetItems() {
    var xhttp = new XMLHttpRequest();
    xhttp.open("GET", url, true);
    xhttp.send();
    xhttp.onreadystatechange = function() {
        if (this.readyState == 4 && this.status == 200) {

            container.innerHTML = ""

            var response = this.responseText;
            if(count==1) {
                console.log(response)
            }

            var respJson = JSON.parse(response);
            for( let i in respJson ){

                var tr = document.createElement("tr")
                var td1 = document.createElement("td")
                var td2 = document.createElement("td")
                var p1 = document.createElement('p')
                var p2 = document.createElement('p')
                var textNode2 = document.createTextNode( calcBar(respJson[i].Percentage) + " " + bytesToSize(respJson[i].DownloadedByte) + " / " + bytesToSize(respJson[i].Size) + " " + respJson[i].Status)
                var textNode1 = document.createTextNode(respJson[i].NameFile)


                tr.setAttribute("id", i)
                p1.appendChild(textNode1)
                p2.appendChild(textNode2)
                td1.appendChild(p1)
                td2.appendChild(p2)
                tr.appendChild(td1)
                tr.appendChild(td2)

                container.appendChild(tr)

            }
        }
    };

}, 1000);

function calcBar(percent) {
    var space = 100
    if(percent == "") {
        percent = "00.00%"
    }
    var str = percent + "["
    percent2 = percent.replace('%','');
    var num = parseInt(percent2)

    for(var i=0; i<num; i++) {
        str += "="
        space--
    }

    str += ">"
    space--

    for (var i=0; i<space; i++){
        str += "-"
    }

    str += "] "

    return str
}

function bytesToSize(bytes) {
    var sizes = ['Bytes', 'KB', 'MB', 'GB', 'TB'];
    if (bytes == 0) return '0 Byte';
    var i = parseInt(Math.floor(Math.log(bytes) / Math.log(1024)));
    return Math.round(bytes / Math.pow(1024, i), 2) + ' ' + sizes[i];
}
