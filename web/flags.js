// Row-major order in the 16-column flag-icons 7.3.2 sprite.
const flagCodes = (
  "ad ae af ag ai al am ao aq ar as at au aw ax az" + " " +
  "ba bb bd be bf bg bh bi bj bl bm bn bo bq br bs" + " " +
  "bt bv bw by bz ca cc cd cf cg ch ci ck cl cm cn" + " " +
  "co cr cu cv cw cx cy cz de dj dk dm do dz ec ee" + " " +
  "eg eh er es et fi fj fk fm fo fr ga gb gd ge gf" + " " +
  "gg gh gi gl gm gn gp gq gr gs gt gu gw gy hk hm" + " " +
  "hn hr ht hu id ie il im in io iq ir is it je jm" + " " +
  "jo jp ke kg kh ki km kn kp kr kw ky kz la lb lc" + " " +
  "li lk lr ls lt lu lv ly ma mc md me mf mg mh mk" + " " +
  "ml mm mn mo mp mq mr ms mt mu mv mw mx my mz na" + " " +
  "nc ne nf ng ni nl no np nr nu nz om pa pe pf pg" + " " +
  "ph pk pl pm pn pr ps pt pw py qa re ro rs ru rw" + " " +
  "sa sb sc sd se sg sh si sj sk sl sm sn so sr ss" + " " +
  "st sv sx sy sz tc td tf tg th tj tk tl tm tn to" + " " +
  "tr tt tv tw tz ua ug um us uy uz va vc ve vg vi" + " " +
  "vn vu wf ws xk ye yt za zm zw"
).split(" ");

const flagRegions = new Map();
for (const locale of ["zh-Hans", "zh-Hant", "en"]) {
  const names = new Intl.DisplayNames(locale, {type:"region"});
  flagCodes.forEach((code,index) => {
    flagRegions.set(names.of(code.toUpperCase()).toLowerCase(), index);
    flagRegions.set(code, index);
  });
}
for (const [name,code] of Object.entries({
  "中国香港":"hk", "香港":"hk", "中國香港":"hk", "中国澳门":"mo", "澳門":"mo", "澳门":"mo", "中國澳門":"mo",
  "中国台湾":"tw", "中國台灣":"tw", "中国 台湾":"tw", "中国 香港":"hk", "中国 澳门":"mo",
  "俄罗斯联邦":"ru", "土耳其":"tr", "捷克共和国":"cz", "美利坚合众国":"us", "uk":"gb", "usa":"us"
})) flagRegions.set(name, flagCodes.indexOf(code));

function regionFlag(location) {
  // Ping0 returns the country followed by space-separated regional details.
  const words = (location||"").trim().toLowerCase().split(/\s+/);
  while (words.length) {
    const index = flagRegions.get(words.join(" "));
    if (index !== undefined) {
      const x = -(index % 16) * 16, y = -Math.floor(index / 16) * 12;
      return `<span class="region-flag" aria-hidden="true" style="background-position:${x}px ${y}px"></span>`;
    }
    words.pop();
  }
  return "";
}
