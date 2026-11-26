"use strict";

const byId = id => document.getElementById(id);
const translations = [...document.querySelectorAll("[data-en]")].map(element => ({
  element, zh: element.textContent, en: element.dataset.en
}));
const language = byId("language");
const copy = byId("copy");
let english = new URLSearchParams(location.search).get("lang") === "en";
let copyTimer;

function setLanguage() {
  clearTimeout(copyTimer);
  byId("feedback").textContent = "";
  for (const label of translations) label.element.textContent = english ? label.en : label.zh;
  document.documentElement.lang = english ? "en" : "zh-CN";
  document.title = english ? "EasyNodeDeck · Self-hosted node management" : "EasyNodeDeck 易节点 · 自托管节点管理面板";
  language.textContent = english ? "中文" : "EN";
  language.setAttribute("aria-label", english ? "切换为中文" : "Switch to English");
  document.querySelector(".doc").href = "https://github.com/sl2782087/easynodedeck/blob/main/" + (english ? "README.en.md" : "README.md");
  byId("ports").href = document.querySelector(".doc").href + (english ? "#port-reference" : "#端口清单");
  const screenshot = byId("screenshot");
  screenshot.src = "./assets/panel-preview" + (english ? "-en" : "") + ".webp";
  screenshot.alt = english ? "Node management panel showing status, egress regions and protocols" : "节点管理面板，显示节点状态、出口地区及协议";
  byId("full-preview").href = screenshot.src;
}

language.onclick = () => {
  english = !english;
  const url = new URL(location.href);
  if (english) url.searchParams.set("lang","en");
  else url.searchParams.delete("lang");
  history.replaceState(null,"",url);
  setLanguage();
};
language.hidden = copy.hidden = false;
setLanguage();

for (const id of ["nodes","subs"]) {
  byId(id+"-tab").onclick = () => {
    for (const panel of ["nodes","subs"]) {
      byId(panel).hidden = panel !== id;
      byId(panel+"-tab").setAttribute("aria-pressed",String(panel === id));
    }
  };
}

copy.onclick = async () => {
  clearTimeout(copyTimer);
  try {
    await navigator.clipboard.writeText(byId("command").textContent);
    copy.textContent = english ? "Copied" : "已复制";
    byId("feedback").textContent = copy.textContent;
  } catch {
    const range = document.createRange();
    range.selectNodeContents(byId("command"));
    const selection = window.getSelection();
    selection.removeAllRanges();
    selection.addRange(range);
    copy.textContent = english ? "Copy selection" : "请手动复制选区";
    byId("feedback").textContent = copy.textContent;
  }
  copyTimer = setTimeout(() => { copy.textContent = english ? "Copy" : "复制命令"; }, 3000);
};
