// Spec :
// - text
// - path
// - subtext
// - timer

// Get the Date 
const getStamps = (ms) => {
    days = Math.floor(ms / (1000 * 60 * 60 * 24))
    hours = Math.floor(ms / (1000 * 60 * 60)) - (days * 24)
    minutes = Math.floor(ms / (1000 * 60)) - (hours * 60) - (days * 24 * 60)
    seconds = Math.floor(ms / 1000) - (minutes * 60) - (hours * 60 * 60) - (days * 24 * 60 * 60)
    return { "days": days, "hours": hours, "minutes": minutes, "seconds": seconds }
}
const getDiff = (nowDate) => {
    DAY_IN_MS = 24 * 60 * 60 * 1000
    if (nowDate == null) {
        var nowDate = new Date()
    }
    let curr_year = nowDate.getFullYear()
    let now = nowDate
    let target = new Date(`Dec 20, ${curr_year} 13:15:00`)
    let delta = now - target
    let status = ""
    // Pre Condition
    if (delta < 0) {
        status = "pre"
    }
    // Post Condition
    else if (delta > DAY_IN_MS) {
        status = "post"
    }
    // In Condition
    else if (delta > 0) {
        status = "during"
    }
    return {
        "status": status, "diff": getStamps(Math.abs(delta))
    }
}
const runTests = () => {
    // Test Scenarios
    let curr_year = new Date().getFullYear()
    // Pre Condition
    console.log("Testing Pre Condition")
    console.log("Expected : status = pre, diff = ")
    let date1 = new Date(`Dec 18, ${curr_year} 11:00:00`)
    let test1 = getDiff(date1)
    console.log(`status = ${test1.status}`)
    // During Condition
    console.log("Testing During Condition")
    console.log("Expected : status = during, diff = ")
    let date2 = new Date(`Dec 20, ${curr_year} 14:00:00`)
    let test2 = getDiff(date2)
    console.log(`status = ${test2.status}`)// Post Condition
    // Post Condition
    console.log("Testing Post Condition")
    console.log("Expected : status = post, diff = ")
    let date3 = new Date(`Dec 21, ${curr_year} 13:20:00`)
    let test3 = getDiff(date3)
    console.log(`status = ${test3.status}`)
}
// Display Text, subtext, timer(if not during), path

let pre = {
    "text": "BIRTHDAY AFTER",
    "subtext": "Intazar do kya appu ajhai alik time 6a",
    "timer": true,
    "path": "pre"
}
let during = {
    "text": "HAPPY BIRTHDAY MUTU",
    "subtext": "Janmadin ko dherai dherai subhakamana mero mutu. Aune din haru timro ajhai mitho ra maya le bhareko hoss.",
    "timer": false,
    "path": "during"
}
let post = {
    "text": "SAKYO BIRTHDAY ",
    "subtext": "LAAAAA! ajhai ek barsa badhyo ! Budho huna atis hai, ma jastai tanneri basna mehenat gar ajhai la !",
    "timer": true,
    "path": "post"
}
const changeImage = (path) => {
    let image = document.getElementById("gif")
    counter = Math.floor(Math.random() * 3) + 1
    if (!image.src.includes("gif")) {
        console.log("Image is empty")
        image.src = "./assets/" + path + "/" + counter.toString() + ".gif"
    }
    if (new Date().getSeconds() % 5 == 0) {
        console.log("Changing Image")
        image.src = "./assets/" + path + "/" + counter.toString() + ".gif"
    }
}
const updatePage = () => {
    let text = document.getElementById("text")
    let subtext = document.getElementById("subtext")
    let timer = document.getElementById("timer")
    let state = getDiff()
    let countdown = `${state.diff.days}d ${state.diff.hours}h ${state.diff.minutes}m ${state.diff.seconds}s`
    if (state.status == "pre") {
        text.innerHTML = pre.text
        subtext.innerHTML = pre.subtext
        timer.innerHTML = countdown
        changeImage(pre.path)
    }
    else if (state.status == "during") {
        text.innerHTML = during.text
        subtext.innerHTML = during.subtext
        timer.setAttribute("class", "hidden")
        changeImage(during.path)
        let background = document.getElementById("background")
        background.setAttribute("class", "img-background")
        subtext.setAttribute("class", "text-center text-2xl text-white")
    }
    else if (state.status == "post") {
        text.innerHTML = post.text
        subtext.innerHTML = post.subtext
        timer.innerHTML = countdown + " ago."
        changeImage(post.path)
    }
}
document.addEventListener("DOMContentLoaded", () => {
    setInterval(() => {
        updatePage()
    }, 1000)
    updatePage()
})
