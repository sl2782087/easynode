"use strict";

// One open list across the page and its modal. Keep form values on the trigger.
let openPicker = null;

function closePicker(restoreFocus = false) {
  if (!openPicker) return;
  const {trigger, menu} = openPicker;
  openPicker = null;
  menu.remove();
  trigger.setAttribute("aria-expanded", "false");
  trigger.removeAttribute("aria-controls");
  if (restoreFocus) trigger.focus({preventScroll:true});
}

function configurePicker(trigger, label, choices, value) {
  trigger.classList.add("picker");
  trigger.setAttribute("aria-haspopup", "listbox");
  trigger.setAttribute("aria-expanded", "false");
  function setValue(next) {
    const choice = choices.find(item => item.value === next);
    trigger.value = choice.value;
    trigger.textContent = choice.label;
    trigger.setAttribute("aria-label", label+" · "+choice.label);
  }
  function open() {
    if (openPicker?.trigger === trigger) { closePicker(true); return; }
    closePicker();
    const menu = document.createElement("div");
    menu.className = "picker-menu";
    menu.id = "active-picker-options";
    menu.setAttribute("role", "listbox");
    menu.setAttribute("aria-label", label);
    for (const choice of choices) {
      const option = document.createElement("button");
      option.type = "button";
      option.tabIndex = -1;
      option.setAttribute("role", "option");
      option.setAttribute("aria-selected", String(choice.value === trigger.value));
      option.textContent = choice.label;
      option.disabled = Boolean(choice.disabled);
      option.onclick = () => {
        setValue(choice.value);
        closePicker(true);
        trigger.dispatchEvent(new Event("change", {bubbles:true}));
      };
      menu.append(option);
    }
    openPicker = {trigger, menu};
    trigger.setAttribute("aria-expanded", "true");
    trigger.setAttribute("aria-controls", menu.id);
    if (typeof menu.showPopover === "function") {
      menu.setAttribute("popover", "manual");
      (trigger.closest("dialog") || document.body).append(menu);
      menu.showPopover();
      const rect = trigger.getBoundingClientRect();
      const gap = 5;
      const width = Math.min(innerWidth-16, Math.max(148, rect.width));
      menu.style.width = width+"px";
      const below = innerHeight-rect.bottom-gap-8;
      const above = rect.top-gap-8;
      const upward = below < menu.offsetHeight && above > below;
      menu.style.maxHeight = Math.max(1, upward ? above : below)+"px";
      menu.style.left = Math.max(8, Math.min(rect.left,innerWidth-width-8))+"px";
      menu.style.top = (upward ? rect.top-gap-menu.offsetHeight : rect.bottom+gap)+"px";
    } else {
      trigger.parentElement.after(menu);
    }
    menu.querySelector('[aria-selected="true"]').focus({preventScroll:true});
    menu.onkeydown = event => {
      const options = [...menu.querySelectorAll("button:enabled")];
      const i = options.indexOf(document.activeElement);
      const last = options.length-1;
      const index = {ArrowDown:Math.min(i+1,last),ArrowUp:Math.max(i-1,0),Home:0,End:last}[event.key];
      if (index !== undefined) {
        event.preventDefault(); options[index].focus();
      } else if (event.key === "Escape") {
        event.preventDefault(); event.stopPropagation(); closePicker(true);
      } else if (event.key === "Tab") {
        closePicker(true);
      }
    };
  }
  setValue(value);
  trigger.onclick = open;
  trigger.onkeydown = event => {
    if (event.key === "ArrowDown" || event.key === "ArrowUp") {
      event.preventDefault(); open();
    }
  };
}

document.addEventListener("pointerdown", event => {
  if (openPicker && !openPicker.menu.contains(event.target) && !openPicker.trigger.contains(event.target)) closePicker();
},true);
document.addEventListener("scroll", event => {
  if (openPicker && !openPicker.menu.contains(event.target)) closePicker();
},true);
window.addEventListener("resize", () => closePicker(true));
