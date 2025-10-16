var __defProp = Object.defineProperty;
var __defNormalProp = (obj, key, value) => key in obj ? __defProp(obj, key, { enumerable: true, configurable: true, writable: true, value }) : obj[key] = value;
var __publicField = (obj, key, value) => __defNormalProp(obj, typeof key !== "symbol" ? key + "" : key, value);

// node_modules/@lit/reactive-element/css-tag.js
var t = globalThis;
var e = t.ShadowRoot && (void 0 === t.ShadyCSS || t.ShadyCSS.nativeShadow) && "adoptedStyleSheets" in Document.prototype && "replace" in CSSStyleSheet.prototype;
var s = Symbol();
var o = /* @__PURE__ */ new WeakMap();
var n = class {
  constructor(t4, e6, o5) {
    if (this._$cssResult$ = true, o5 !== s) throw Error("CSSResult is not constructable. Use `unsafeCSS` or `css` instead.");
    this.cssText = t4, this.t = e6;
  }
  get styleSheet() {
    let t4 = this.o;
    const s4 = this.t;
    if (e && void 0 === t4) {
      const e6 = void 0 !== s4 && 1 === s4.length;
      e6 && (t4 = o.get(s4)), void 0 === t4 && ((this.o = t4 = new CSSStyleSheet()).replaceSync(this.cssText), e6 && o.set(s4, t4));
    }
    return t4;
  }
  toString() {
    return this.cssText;
  }
};
var r = (t4) => new n("string" == typeof t4 ? t4 : t4 + "", void 0, s);
var S = (s4, o5) => {
  if (e) s4.adoptedStyleSheets = o5.map(((t4) => t4 instanceof CSSStyleSheet ? t4 : t4.styleSheet));
  else for (const e6 of o5) {
    const o6 = document.createElement("style"), n4 = t.litNonce;
    void 0 !== n4 && o6.setAttribute("nonce", n4), o6.textContent = e6.cssText, s4.appendChild(o6);
  }
};
var c = e ? (t4) => t4 : (t4) => t4 instanceof CSSStyleSheet ? ((t5) => {
  let e6 = "";
  for (const s4 of t5.cssRules) e6 += s4.cssText;
  return r(e6);
})(t4) : t4;

// node_modules/@lit/reactive-element/reactive-element.js
var { is: i2, defineProperty: e2, getOwnPropertyDescriptor: h, getOwnPropertyNames: r2, getOwnPropertySymbols: o2, getPrototypeOf: n2 } = Object;
var a = globalThis;
var c2 = a.trustedTypes;
var l = c2 ? c2.emptyScript : "";
var p = a.reactiveElementPolyfillSupport;
var d = (t4, s4) => t4;
var u = { toAttribute(t4, s4) {
  switch (s4) {
    case Boolean:
      t4 = t4 ? l : null;
      break;
    case Object:
    case Array:
      t4 = null == t4 ? t4 : JSON.stringify(t4);
  }
  return t4;
}, fromAttribute(t4, s4) {
  let i6 = t4;
  switch (s4) {
    case Boolean:
      i6 = null !== t4;
      break;
    case Number:
      i6 = null === t4 ? null : Number(t4);
      break;
    case Object:
    case Array:
      try {
        i6 = JSON.parse(t4);
      } catch (t5) {
        i6 = null;
      }
  }
  return i6;
} };
var f = (t4, s4) => !i2(t4, s4);
var b = { attribute: true, type: String, converter: u, reflect: false, useDefault: false, hasChanged: f };
var _a, _b;
(_a = Symbol.metadata) != null ? _a : Symbol.metadata = Symbol("metadata"), (_b = a.litPropertyMetadata) != null ? _b : a.litPropertyMetadata = /* @__PURE__ */ new WeakMap();
var y = class extends HTMLElement {
  static addInitializer(t4) {
    var _a6;
    this._$Ei(), ((_a6 = this.l) != null ? _a6 : this.l = []).push(t4);
  }
  static get observedAttributes() {
    return this.finalize(), this._$Eh && [...this._$Eh.keys()];
  }
  static createProperty(t4, s4 = b) {
    if (s4.state && (s4.attribute = false), this._$Ei(), this.prototype.hasOwnProperty(t4) && ((s4 = Object.create(s4)).wrapped = true), this.elementProperties.set(t4, s4), !s4.noAccessor) {
      const i6 = Symbol(), h3 = this.getPropertyDescriptor(t4, i6, s4);
      void 0 !== h3 && e2(this.prototype, t4, h3);
    }
  }
  static getPropertyDescriptor(t4, s4, i6) {
    var _a6;
    const { get: e6, set: r4 } = (_a6 = h(this.prototype, t4)) != null ? _a6 : { get() {
      return this[s4];
    }, set(t5) {
      this[s4] = t5;
    } };
    return { get: e6, set(s5) {
      const h3 = e6 == null ? void 0 : e6.call(this);
      r4 == null ? void 0 : r4.call(this, s5), this.requestUpdate(t4, h3, i6);
    }, configurable: true, enumerable: true };
  }
  static getPropertyOptions(t4) {
    var _a6;
    return (_a6 = this.elementProperties.get(t4)) != null ? _a6 : b;
  }
  static _$Ei() {
    if (this.hasOwnProperty(d("elementProperties"))) return;
    const t4 = n2(this);
    t4.finalize(), void 0 !== t4.l && (this.l = [...t4.l]), this.elementProperties = new Map(t4.elementProperties);
  }
  static finalize() {
    if (this.hasOwnProperty(d("finalized"))) return;
    if (this.finalized = true, this._$Ei(), this.hasOwnProperty(d("properties"))) {
      const t5 = this.properties, s4 = [...r2(t5), ...o2(t5)];
      for (const i6 of s4) this.createProperty(i6, t5[i6]);
    }
    const t4 = this[Symbol.metadata];
    if (null !== t4) {
      const s4 = litPropertyMetadata.get(t4);
      if (void 0 !== s4) for (const [t5, i6] of s4) this.elementProperties.set(t5, i6);
    }
    this._$Eh = /* @__PURE__ */ new Map();
    for (const [t5, s4] of this.elementProperties) {
      const i6 = this._$Eu(t5, s4);
      void 0 !== i6 && this._$Eh.set(i6, t5);
    }
    this.elementStyles = this.finalizeStyles(this.styles);
  }
  static finalizeStyles(s4) {
    const i6 = [];
    if (Array.isArray(s4)) {
      const e6 = new Set(s4.flat(1 / 0).reverse());
      for (const s5 of e6) i6.unshift(c(s5));
    } else void 0 !== s4 && i6.push(c(s4));
    return i6;
  }
  static _$Eu(t4, s4) {
    const i6 = s4.attribute;
    return false === i6 ? void 0 : "string" == typeof i6 ? i6 : "string" == typeof t4 ? t4.toLowerCase() : void 0;
  }
  constructor() {
    super(), this._$Ep = void 0, this.isUpdatePending = false, this.hasUpdated = false, this._$Em = null, this._$Ev();
  }
  _$Ev() {
    var _a6;
    this._$ES = new Promise(((t4) => this.enableUpdating = t4)), this._$AL = /* @__PURE__ */ new Map(), this._$E_(), this.requestUpdate(), (_a6 = this.constructor.l) == null ? void 0 : _a6.forEach(((t4) => t4(this)));
  }
  addController(t4) {
    var _a6, _b2;
    ((_a6 = this._$EO) != null ? _a6 : this._$EO = /* @__PURE__ */ new Set()).add(t4), void 0 !== this.renderRoot && this.isConnected && ((_b2 = t4.hostConnected) == null ? void 0 : _b2.call(t4));
  }
  removeController(t4) {
    var _a6;
    (_a6 = this._$EO) == null ? void 0 : _a6.delete(t4);
  }
  _$E_() {
    const t4 = /* @__PURE__ */ new Map(), s4 = this.constructor.elementProperties;
    for (const i6 of s4.keys()) this.hasOwnProperty(i6) && (t4.set(i6, this[i6]), delete this[i6]);
    t4.size > 0 && (this._$Ep = t4);
  }
  createRenderRoot() {
    var _a6;
    const t4 = (_a6 = this.shadowRoot) != null ? _a6 : this.attachShadow(this.constructor.shadowRootOptions);
    return S(t4, this.constructor.elementStyles), t4;
  }
  connectedCallback() {
    var _a6, _b2;
    (_a6 = this.renderRoot) != null ? _a6 : this.renderRoot = this.createRenderRoot(), this.enableUpdating(true), (_b2 = this._$EO) == null ? void 0 : _b2.forEach(((t4) => {
      var _a7;
      return (_a7 = t4.hostConnected) == null ? void 0 : _a7.call(t4);
    }));
  }
  enableUpdating(t4) {
  }
  disconnectedCallback() {
    var _a6;
    (_a6 = this._$EO) == null ? void 0 : _a6.forEach(((t4) => {
      var _a7;
      return (_a7 = t4.hostDisconnected) == null ? void 0 : _a7.call(t4);
    }));
  }
  attributeChangedCallback(t4, s4, i6) {
    this._$AK(t4, i6);
  }
  _$ET(t4, s4) {
    var _a6;
    const i6 = this.constructor.elementProperties.get(t4), e6 = this.constructor._$Eu(t4, i6);
    if (void 0 !== e6 && true === i6.reflect) {
      const h3 = (void 0 !== ((_a6 = i6.converter) == null ? void 0 : _a6.toAttribute) ? i6.converter : u).toAttribute(s4, i6.type);
      this._$Em = t4, null == h3 ? this.removeAttribute(e6) : this.setAttribute(e6, h3), this._$Em = null;
    }
  }
  _$AK(t4, s4) {
    var _a6, _b2, _c;
    const i6 = this.constructor, e6 = i6._$Eh.get(t4);
    if (void 0 !== e6 && this._$Em !== e6) {
      const t5 = i6.getPropertyOptions(e6), h3 = "function" == typeof t5.converter ? { fromAttribute: t5.converter } : void 0 !== ((_a6 = t5.converter) == null ? void 0 : _a6.fromAttribute) ? t5.converter : u;
      this._$Em = e6;
      const r4 = h3.fromAttribute(s4, t5.type);
      this[e6] = (_c = r4 != null ? r4 : (_b2 = this._$Ej) == null ? void 0 : _b2.get(e6)) != null ? _c : r4, this._$Em = null;
    }
  }
  requestUpdate(t4, s4, i6) {
    var _a6, _b2;
    if (void 0 !== t4) {
      const e6 = this.constructor, h3 = this[t4];
      if (i6 != null ? i6 : i6 = e6.getPropertyOptions(t4), !(((_a6 = i6.hasChanged) != null ? _a6 : f)(h3, s4) || i6.useDefault && i6.reflect && h3 === ((_b2 = this._$Ej) == null ? void 0 : _b2.get(t4)) && !this.hasAttribute(e6._$Eu(t4, i6)))) return;
      this.C(t4, s4, i6);
    }
    false === this.isUpdatePending && (this._$ES = this._$EP());
  }
  C(t4, s4, { useDefault: i6, reflect: e6, wrapped: h3 }, r4) {
    var _a6, _b2, _c;
    i6 && !((_a6 = this._$Ej) != null ? _a6 : this._$Ej = /* @__PURE__ */ new Map()).has(t4) && (this._$Ej.set(t4, (_b2 = r4 != null ? r4 : s4) != null ? _b2 : this[t4]), true !== h3 || void 0 !== r4) || (this._$AL.has(t4) || (this.hasUpdated || i6 || (s4 = void 0), this._$AL.set(t4, s4)), true === e6 && this._$Em !== t4 && ((_c = this._$Eq) != null ? _c : this._$Eq = /* @__PURE__ */ new Set()).add(t4));
  }
  async _$EP() {
    this.isUpdatePending = true;
    try {
      await this._$ES;
    } catch (t5) {
      Promise.reject(t5);
    }
    const t4 = this.scheduleUpdate();
    return null != t4 && await t4, !this.isUpdatePending;
  }
  scheduleUpdate() {
    return this.performUpdate();
  }
  performUpdate() {
    var _a6, _b2;
    if (!this.isUpdatePending) return;
    if (!this.hasUpdated) {
      if ((_a6 = this.renderRoot) != null ? _a6 : this.renderRoot = this.createRenderRoot(), this._$Ep) {
        for (const [t6, s5] of this._$Ep) this[t6] = s5;
        this._$Ep = void 0;
      }
      const t5 = this.constructor.elementProperties;
      if (t5.size > 0) for (const [s5, i6] of t5) {
        const { wrapped: t6 } = i6, e6 = this[s5];
        true !== t6 || this._$AL.has(s5) || void 0 === e6 || this.C(s5, void 0, i6, e6);
      }
    }
    let t4 = false;
    const s4 = this._$AL;
    try {
      t4 = this.shouldUpdate(s4), t4 ? (this.willUpdate(s4), (_b2 = this._$EO) == null ? void 0 : _b2.forEach(((t5) => {
        var _a7;
        return (_a7 = t5.hostUpdate) == null ? void 0 : _a7.call(t5);
      })), this.update(s4)) : this._$EM();
    } catch (s5) {
      throw t4 = false, this._$EM(), s5;
    }
    t4 && this._$AE(s4);
  }
  willUpdate(t4) {
  }
  _$AE(t4) {
    var _a6;
    (_a6 = this._$EO) == null ? void 0 : _a6.forEach(((t5) => {
      var _a7;
      return (_a7 = t5.hostUpdated) == null ? void 0 : _a7.call(t5);
    })), this.hasUpdated || (this.hasUpdated = true, this.firstUpdated(t4)), this.updated(t4);
  }
  _$EM() {
    this._$AL = /* @__PURE__ */ new Map(), this.isUpdatePending = false;
  }
  get updateComplete() {
    return this.getUpdateComplete();
  }
  getUpdateComplete() {
    return this._$ES;
  }
  shouldUpdate(t4) {
    return true;
  }
  update(t4) {
    this._$Eq && (this._$Eq = this._$Eq.forEach(((t5) => this._$ET(t5, this[t5])))), this._$EM();
  }
  updated(t4) {
  }
  firstUpdated(t4) {
  }
};
var _a2;
y.elementStyles = [], y.shadowRootOptions = { mode: "open" }, y[d("elementProperties")] = /* @__PURE__ */ new Map(), y[d("finalized")] = /* @__PURE__ */ new Map(), p == null ? void 0 : p({ ReactiveElement: y }), ((_a2 = a.reactiveElementVersions) != null ? _a2 : a.reactiveElementVersions = []).push("2.1.1");

// node_modules/lit-html/lit-html.js
var t2 = globalThis;
var i3 = t2.trustedTypes;
var s2 = i3 ? i3.createPolicy("lit-html", { createHTML: (t4) => t4 }) : void 0;
var e3 = "$lit$";
var h2 = `lit$${Math.random().toFixed(9).slice(2)}$`;
var o3 = "?" + h2;
var n3 = `<${o3}>`;
var r3 = document;
var l2 = () => r3.createComment("");
var c3 = (t4) => null === t4 || "object" != typeof t4 && "function" != typeof t4;
var a2 = Array.isArray;
var u2 = (t4) => a2(t4) || "function" == typeof (t4 == null ? void 0 : t4[Symbol.iterator]);
var d2 = "[ 	\n\f\r]";
var f2 = /<(?:(!--|\/[^a-zA-Z])|(\/?[a-zA-Z][^>\s]*)|(\/?$))/g;
var v = /-->/g;
var _ = />/g;
var m = RegExp(`>|${d2}(?:([^\\s"'>=/]+)(${d2}*=${d2}*(?:[^ 	
\f\r"'\`<>=]|("|')|))|$)`, "g");
var p2 = /'/g;
var g = /"/g;
var $ = /^(?:script|style|textarea|title)$/i;
var y2 = (t4) => (i6, ...s4) => ({ _$litType$: t4, strings: i6, values: s4 });
var x = y2(1);
var b2 = y2(2);
var w = y2(3);
var T = Symbol.for("lit-noChange");
var E = Symbol.for("lit-nothing");
var A = /* @__PURE__ */ new WeakMap();
var C = r3.createTreeWalker(r3, 129);
function P(t4, i6) {
  if (!a2(t4) || !t4.hasOwnProperty("raw")) throw Error("invalid template strings array");
  return void 0 !== s2 ? s2.createHTML(i6) : i6;
}
var V = (t4, i6) => {
  const s4 = t4.length - 1, o5 = [];
  let r4, l3 = 2 === i6 ? "<svg>" : 3 === i6 ? "<math>" : "", c4 = f2;
  for (let i7 = 0; i7 < s4; i7++) {
    const s5 = t4[i7];
    let a3, u3, d3 = -1, y3 = 0;
    for (; y3 < s5.length && (c4.lastIndex = y3, u3 = c4.exec(s5), null !== u3); ) y3 = c4.lastIndex, c4 === f2 ? "!--" === u3[1] ? c4 = v : void 0 !== u3[1] ? c4 = _ : void 0 !== u3[2] ? ($.test(u3[2]) && (r4 = RegExp("</" + u3[2], "g")), c4 = m) : void 0 !== u3[3] && (c4 = m) : c4 === m ? ">" === u3[0] ? (c4 = r4 != null ? r4 : f2, d3 = -1) : void 0 === u3[1] ? d3 = -2 : (d3 = c4.lastIndex - u3[2].length, a3 = u3[1], c4 = void 0 === u3[3] ? m : '"' === u3[3] ? g : p2) : c4 === g || c4 === p2 ? c4 = m : c4 === v || c4 === _ ? c4 = f2 : (c4 = m, r4 = void 0);
    const x2 = c4 === m && t4[i7 + 1].startsWith("/>") ? " " : "";
    l3 += c4 === f2 ? s5 + n3 : d3 >= 0 ? (o5.push(a3), s5.slice(0, d3) + e3 + s5.slice(d3) + h2 + x2) : s5 + h2 + (-2 === d3 ? i7 : x2);
  }
  return [P(t4, l3 + (t4[s4] || "<?>") + (2 === i6 ? "</svg>" : 3 === i6 ? "</math>" : "")), o5];
};
var N = class _N {
  constructor({ strings: t4, _$litType$: s4 }, n4) {
    let r4;
    this.parts = [];
    let c4 = 0, a3 = 0;
    const u3 = t4.length - 1, d3 = this.parts, [f3, v2] = V(t4, s4);
    if (this.el = _N.createElement(f3, n4), C.currentNode = this.el.content, 2 === s4 || 3 === s4) {
      const t5 = this.el.content.firstChild;
      t5.replaceWith(...t5.childNodes);
    }
    for (; null !== (r4 = C.nextNode()) && d3.length < u3; ) {
      if (1 === r4.nodeType) {
        if (r4.hasAttributes()) for (const t5 of r4.getAttributeNames()) if (t5.endsWith(e3)) {
          const i6 = v2[a3++], s5 = r4.getAttribute(t5).split(h2), e6 = /([.?@])?(.*)/.exec(i6);
          d3.push({ type: 1, index: c4, name: e6[2], strings: s5, ctor: "." === e6[1] ? H : "?" === e6[1] ? I : "@" === e6[1] ? L : k }), r4.removeAttribute(t5);
        } else t5.startsWith(h2) && (d3.push({ type: 6, index: c4 }), r4.removeAttribute(t5));
        if ($.test(r4.tagName)) {
          const t5 = r4.textContent.split(h2), s5 = t5.length - 1;
          if (s5 > 0) {
            r4.textContent = i3 ? i3.emptyScript : "";
            for (let i6 = 0; i6 < s5; i6++) r4.append(t5[i6], l2()), C.nextNode(), d3.push({ type: 2, index: ++c4 });
            r4.append(t5[s5], l2());
          }
        }
      } else if (8 === r4.nodeType) if (r4.data === o3) d3.push({ type: 2, index: c4 });
      else {
        let t5 = -1;
        for (; -1 !== (t5 = r4.data.indexOf(h2, t5 + 1)); ) d3.push({ type: 7, index: c4 }), t5 += h2.length - 1;
      }
      c4++;
    }
  }
  static createElement(t4, i6) {
    const s4 = r3.createElement("template");
    return s4.innerHTML = t4, s4;
  }
};
function S2(t4, i6, s4 = t4, e6) {
  var _a6, _b2, _c;
  if (i6 === T) return i6;
  let h3 = void 0 !== e6 ? (_a6 = s4._$Co) == null ? void 0 : _a6[e6] : s4._$Cl;
  const o5 = c3(i6) ? void 0 : i6._$litDirective$;
  return (h3 == null ? void 0 : h3.constructor) !== o5 && ((_b2 = h3 == null ? void 0 : h3._$AO) == null ? void 0 : _b2.call(h3, false), void 0 === o5 ? h3 = void 0 : (h3 = new o5(t4), h3._$AT(t4, s4, e6)), void 0 !== e6 ? ((_c = s4._$Co) != null ? _c : s4._$Co = [])[e6] = h3 : s4._$Cl = h3), void 0 !== h3 && (i6 = S2(t4, h3._$AS(t4, i6.values), h3, e6)), i6;
}
var M = class {
  constructor(t4, i6) {
    this._$AV = [], this._$AN = void 0, this._$AD = t4, this._$AM = i6;
  }
  get parentNode() {
    return this._$AM.parentNode;
  }
  get _$AU() {
    return this._$AM._$AU;
  }
  u(t4) {
    var _a6;
    const { el: { content: i6 }, parts: s4 } = this._$AD, e6 = ((_a6 = t4 == null ? void 0 : t4.creationScope) != null ? _a6 : r3).importNode(i6, true);
    C.currentNode = e6;
    let h3 = C.nextNode(), o5 = 0, n4 = 0, l3 = s4[0];
    for (; void 0 !== l3; ) {
      if (o5 === l3.index) {
        let i7;
        2 === l3.type ? i7 = new R(h3, h3.nextSibling, this, t4) : 1 === l3.type ? i7 = new l3.ctor(h3, l3.name, l3.strings, this, t4) : 6 === l3.type && (i7 = new z(h3, this, t4)), this._$AV.push(i7), l3 = s4[++n4];
      }
      o5 !== (l3 == null ? void 0 : l3.index) && (h3 = C.nextNode(), o5++);
    }
    return C.currentNode = r3, e6;
  }
  p(t4) {
    let i6 = 0;
    for (const s4 of this._$AV) void 0 !== s4 && (void 0 !== s4.strings ? (s4._$AI(t4, s4, i6), i6 += s4.strings.length - 2) : s4._$AI(t4[i6])), i6++;
  }
};
var R = class _R {
  get _$AU() {
    var _a6, _b2;
    return (_b2 = (_a6 = this._$AM) == null ? void 0 : _a6._$AU) != null ? _b2 : this._$Cv;
  }
  constructor(t4, i6, s4, e6) {
    var _a6;
    this.type = 2, this._$AH = E, this._$AN = void 0, this._$AA = t4, this._$AB = i6, this._$AM = s4, this.options = e6, this._$Cv = (_a6 = e6 == null ? void 0 : e6.isConnected) != null ? _a6 : true;
  }
  get parentNode() {
    let t4 = this._$AA.parentNode;
    const i6 = this._$AM;
    return void 0 !== i6 && 11 === (t4 == null ? void 0 : t4.nodeType) && (t4 = i6.parentNode), t4;
  }
  get startNode() {
    return this._$AA;
  }
  get endNode() {
    return this._$AB;
  }
  _$AI(t4, i6 = this) {
    t4 = S2(this, t4, i6), c3(t4) ? t4 === E || null == t4 || "" === t4 ? (this._$AH !== E && this._$AR(), this._$AH = E) : t4 !== this._$AH && t4 !== T && this._(t4) : void 0 !== t4._$litType$ ? this.$(t4) : void 0 !== t4.nodeType ? this.T(t4) : u2(t4) ? this.k(t4) : this._(t4);
  }
  O(t4) {
    return this._$AA.parentNode.insertBefore(t4, this._$AB);
  }
  T(t4) {
    this._$AH !== t4 && (this._$AR(), this._$AH = this.O(t4));
  }
  _(t4) {
    this._$AH !== E && c3(this._$AH) ? this._$AA.nextSibling.data = t4 : this.T(r3.createTextNode(t4)), this._$AH = t4;
  }
  $(t4) {
    var _a6;
    const { values: i6, _$litType$: s4 } = t4, e6 = "number" == typeof s4 ? this._$AC(t4) : (void 0 === s4.el && (s4.el = N.createElement(P(s4.h, s4.h[0]), this.options)), s4);
    if (((_a6 = this._$AH) == null ? void 0 : _a6._$AD) === e6) this._$AH.p(i6);
    else {
      const t5 = new M(e6, this), s5 = t5.u(this.options);
      t5.p(i6), this.T(s5), this._$AH = t5;
    }
  }
  _$AC(t4) {
    let i6 = A.get(t4.strings);
    return void 0 === i6 && A.set(t4.strings, i6 = new N(t4)), i6;
  }
  k(t4) {
    a2(this._$AH) || (this._$AH = [], this._$AR());
    const i6 = this._$AH;
    let s4, e6 = 0;
    for (const h3 of t4) e6 === i6.length ? i6.push(s4 = new _R(this.O(l2()), this.O(l2()), this, this.options)) : s4 = i6[e6], s4._$AI(h3), e6++;
    e6 < i6.length && (this._$AR(s4 && s4._$AB.nextSibling, e6), i6.length = e6);
  }
  _$AR(t4 = this._$AA.nextSibling, i6) {
    var _a6;
    for ((_a6 = this._$AP) == null ? void 0 : _a6.call(this, false, true, i6); t4 !== this._$AB; ) {
      const i7 = t4.nextSibling;
      t4.remove(), t4 = i7;
    }
  }
  setConnected(t4) {
    var _a6;
    void 0 === this._$AM && (this._$Cv = t4, (_a6 = this._$AP) == null ? void 0 : _a6.call(this, t4));
  }
};
var k = class {
  get tagName() {
    return this.element.tagName;
  }
  get _$AU() {
    return this._$AM._$AU;
  }
  constructor(t4, i6, s4, e6, h3) {
    this.type = 1, this._$AH = E, this._$AN = void 0, this.element = t4, this.name = i6, this._$AM = e6, this.options = h3, s4.length > 2 || "" !== s4[0] || "" !== s4[1] ? (this._$AH = Array(s4.length - 1).fill(new String()), this.strings = s4) : this._$AH = E;
  }
  _$AI(t4, i6 = this, s4, e6) {
    const h3 = this.strings;
    let o5 = false;
    if (void 0 === h3) t4 = S2(this, t4, i6, 0), o5 = !c3(t4) || t4 !== this._$AH && t4 !== T, o5 && (this._$AH = t4);
    else {
      const e7 = t4;
      let n4, r4;
      for (t4 = h3[0], n4 = 0; n4 < h3.length - 1; n4++) r4 = S2(this, e7[s4 + n4], i6, n4), r4 === T && (r4 = this._$AH[n4]), o5 || (o5 = !c3(r4) || r4 !== this._$AH[n4]), r4 === E ? t4 = E : t4 !== E && (t4 += (r4 != null ? r4 : "") + h3[n4 + 1]), this._$AH[n4] = r4;
    }
    o5 && !e6 && this.j(t4);
  }
  j(t4) {
    t4 === E ? this.element.removeAttribute(this.name) : this.element.setAttribute(this.name, t4 != null ? t4 : "");
  }
};
var H = class extends k {
  constructor() {
    super(...arguments), this.type = 3;
  }
  j(t4) {
    this.element[this.name] = t4 === E ? void 0 : t4;
  }
};
var I = class extends k {
  constructor() {
    super(...arguments), this.type = 4;
  }
  j(t4) {
    this.element.toggleAttribute(this.name, !!t4 && t4 !== E);
  }
};
var L = class extends k {
  constructor(t4, i6, s4, e6, h3) {
    super(t4, i6, s4, e6, h3), this.type = 5;
  }
  _$AI(t4, i6 = this) {
    var _a6;
    if ((t4 = (_a6 = S2(this, t4, i6, 0)) != null ? _a6 : E) === T) return;
    const s4 = this._$AH, e6 = t4 === E && s4 !== E || t4.capture !== s4.capture || t4.once !== s4.once || t4.passive !== s4.passive, h3 = t4 !== E && (s4 === E || e6);
    e6 && this.element.removeEventListener(this.name, this, s4), h3 && this.element.addEventListener(this.name, this, t4), this._$AH = t4;
  }
  handleEvent(t4) {
    var _a6, _b2;
    "function" == typeof this._$AH ? this._$AH.call((_b2 = (_a6 = this.options) == null ? void 0 : _a6.host) != null ? _b2 : this.element, t4) : this._$AH.handleEvent(t4);
  }
};
var z = class {
  constructor(t4, i6, s4) {
    this.element = t4, this.type = 6, this._$AN = void 0, this._$AM = i6, this.options = s4;
  }
  get _$AU() {
    return this._$AM._$AU;
  }
  _$AI(t4) {
    S2(this, t4);
  }
};
var j = t2.litHtmlPolyfillSupport;
var _a3;
j == null ? void 0 : j(N, R), ((_a3 = t2.litHtmlVersions) != null ? _a3 : t2.litHtmlVersions = []).push("3.3.1");
var B = (t4, i6, s4) => {
  var _a6, _b2;
  const e6 = (_a6 = s4 == null ? void 0 : s4.renderBefore) != null ? _a6 : i6;
  let h3 = e6._$litPart$;
  if (void 0 === h3) {
    const t5 = (_b2 = s4 == null ? void 0 : s4.renderBefore) != null ? _b2 : null;
    e6._$litPart$ = h3 = new R(i6.insertBefore(l2(), t5), t5, void 0, s4 != null ? s4 : {});
  }
  return h3._$AI(t4), h3;
};

// node_modules/lit-element/lit-element.js
var s3 = globalThis;
var i4 = class extends y {
  constructor() {
    super(...arguments), this.renderOptions = { host: this }, this._$Do = void 0;
  }
  createRenderRoot() {
    var _a6, _b2;
    const t4 = super.createRenderRoot();
    return (_b2 = (_a6 = this.renderOptions).renderBefore) != null ? _b2 : _a6.renderBefore = t4.firstChild, t4;
  }
  update(t4) {
    const r4 = this.render();
    this.hasUpdated || (this.renderOptions.isConnected = this.isConnected), super.update(t4), this._$Do = B(r4, this.renderRoot, this.renderOptions);
  }
  connectedCallback() {
    var _a6;
    super.connectedCallback(), (_a6 = this._$Do) == null ? void 0 : _a6.setConnected(true);
  }
  disconnectedCallback() {
    var _a6;
    super.disconnectedCallback(), (_a6 = this._$Do) == null ? void 0 : _a6.setConnected(false);
  }
  render() {
    return T;
  }
};
var _a4;
i4._$litElement$ = true, i4["finalized"] = true, (_a4 = s3.litElementHydrateSupport) == null ? void 0 : _a4.call(s3, { LitElement: i4 });
var o4 = s3.litElementPolyfillSupport;
o4 == null ? void 0 : o4({ LitElement: i4 });
var _a5;
((_a5 = s3.litElementVersions) != null ? _a5 : s3.litElementVersions = []).push("4.2.1");

// node_modules/lit-html/directive.js
var t3 = { ATTRIBUTE: 1, CHILD: 2, PROPERTY: 3, BOOLEAN_ATTRIBUTE: 4, EVENT: 5, ELEMENT: 6 };
var e4 = (t4) => (...e6) => ({ _$litDirective$: t4, values: e6 });
var i5 = class {
  constructor(t4) {
  }
  get _$AU() {
    return this._$AM._$AU;
  }
  _$AT(t4, e6, i6) {
    this._$Ct = t4, this._$AM = e6, this._$Ci = i6;
  }
  _$AS(t4, e6) {
    return this.update(t4, e6);
  }
  update(t4, e6) {
    return this.render(...e6);
  }
};

// node_modules/lit-html/directives/class-map.js
var e5 = e4(class extends i5 {
  constructor(t4) {
    var _a6;
    if (super(t4), t4.type !== t3.ATTRIBUTE || "class" !== t4.name || ((_a6 = t4.strings) == null ? void 0 : _a6.length) > 2) throw Error("`classMap()` can only be used in the `class` attribute and must be the only part in the attribute.");
  }
  render(t4) {
    return " " + Object.keys(t4).filter(((s4) => t4[s4])).join(" ") + " ";
  }
  update(s4, [i6]) {
    var _a6, _b2;
    if (void 0 === this.st) {
      this.st = /* @__PURE__ */ new Set(), void 0 !== s4.strings && (this.nt = new Set(s4.strings.join(" ").split(/\s/).filter(((t4) => "" !== t4))));
      for (const t4 in i6) i6[t4] && !((_a6 = this.nt) == null ? void 0 : _a6.has(t4)) && this.st.add(t4);
      return this.render(i6);
    }
    const r4 = s4.element.classList;
    for (const t4 of this.st) t4 in i6 || (r4.remove(t4), this.st.delete(t4));
    for (const t4 in i6) {
      const s5 = !!i6[t4];
      s5 === this.st.has(t4) || ((_b2 = this.nt) == null ? void 0 : _b2.has(t4)) || (s5 ? (r4.add(t4), this.st.add(t4)) : (r4.remove(t4), this.st.delete(t4)));
    }
    return T;
  }
});

// client/effect-catalog.ts
var EMPTY_CATALOG = Object.freeze({});
var isRecord = (value) => typeof value === "object" && value !== null && !Array.isArray(value);
var cloneBlocks = (source) => {
  const copy = {};
  for (const [key, value] of Object.entries(source)) {
    copy[key] = value;
  }
  return copy;
};
var normalizeEffectCatalog = (input) => {
  if (input == null) {
    return EMPTY_CATALOG;
  }
  if (!isRecord(input)) {
    throw new Error("Effect catalog must be an object map of entry metadata.");
  }
  const result = {};
  for (const [entryId, entryValue] of Object.entries(input)) {
    if (!isRecord(entryValue)) {
      throw new Error(`Effect catalog entry ${entryId} must be an object.`);
    }
    const { contractId, blocks } = entryValue;
    if (typeof contractId !== "string" || contractId.length === 0) {
      throw new Error(`Effect catalog entry ${entryId} missing contractId.`);
    }
    let normalizedBlocks;
    if (blocks !== void 0) {
      if (!isRecord(blocks)) {
        throw new Error(`Effect catalog entry ${entryId} blocks must be an object.`);
      }
      normalizedBlocks = cloneBlocks(blocks);
    }
    result[entryId] = {
      contractId,
      blocks: Object.freeze(normalizedBlocks != null ? normalizedBlocks : {})
    };
  }
  return Object.freeze(result);
};
var currentCatalog = EMPTY_CATALOG;
var setEffectCatalog = (catalog) => {
  currentCatalog = catalog ? Object.freeze({ ...catalog }) : EMPTY_CATALOG;
};

// client/network.ts
var WebSocketNetworkClient = class {
  constructor(configuration) {
    this.configuration = configuration;
    __publicField(this, "socket", null);
    __publicField(this, "joinResponse", null);
    __publicField(this, "handlers", null);
  }
  async join() {
    await this.disconnect();
    this.joinResponse = null;
    const response = await fetch(this.configuration.joinUrl, {
      method: "POST",
      cache: "no-store"
    });
    if (!response.ok) {
      throw new Error(`Join request failed with status ${response.status}`);
    }
    const payload = await response.json();
    if (!payload || typeof payload !== "object") {
      throw new Error("Join response payload is not an object.");
    }
    const joinPayload = payload;
    if (typeof joinPayload.id !== "string" || joinPayload.id.length === 0) {
      throw new Error("Join response missing player identifier.");
    }
    if (!joinPayload.config || typeof joinPayload.config !== "object") {
      throw new Error("Join response missing world configuration.");
    }
    const config = joinPayload.config;
    if (typeof config.seed !== "string" || config.seed.length === 0) {
      throw new Error("Join response missing world seed.");
    }
    if (typeof joinPayload.ver !== "number") {
      throw new Error("Join response missing protocol version.");
    }
    const effectCatalog = normalizeEffectCatalog(config.effectCatalog);
    if (joinPayload.ver !== this.configuration.protocolVersion) {
      throw new Error(
        `Protocol mismatch: expected ${this.configuration.protocolVersion}, received ${joinPayload.ver}`
      );
    }
    const joinResponse = {
      id: joinPayload.id,
      seed: config.seed,
      protocolVersion: joinPayload.ver,
      effectCatalog
    };
    this.joinResponse = joinResponse;
    return joinResponse;
  }
  async connect(handlers) {
    if (!this.joinResponse) {
      throw new Error("Cannot connect before joining the world.");
    }
    await this.disconnect();
    this.handlers = handlers;
    const socketUrl = this.createWebSocketUrl(this.joinResponse.id);
    const socket = new WebSocket(socketUrl);
    this.socket = socket;
    await new Promise((resolve, reject) => {
      let resolved = false;
      const handleOpen = () => {
        var _a6;
        resolved = true;
        socket.removeEventListener("open", handleOpen);
        (_a6 = handlers.onJoin) == null ? void 0 : _a6.call(handlers, this.joinResponse);
        resolve();
      };
      const handleMessage = (event) => {
        var _a6;
        let messagePayload = event.data;
        let messageType = "unknown";
        if (typeof event.data === "string") {
          try {
            const parsed = JSON.parse(event.data);
            messagePayload = parsed;
            if (parsed && typeof parsed === "object" && "type" in parsed) {
              const candidate = parsed.type;
              if (typeof candidate === "string" && candidate.length > 0) {
                messageType = candidate;
              }
            }
          } catch (error) {
            messagePayload = event.data;
            if (handlers.onError) {
              const cause = error instanceof Error ? error : new Error(String(error));
              handlers.onError(cause);
            }
          }
        }
        const envelope = {
          type: messageType,
          payload: messagePayload,
          receivedAt: Date.now()
        };
        (_a6 = handlers.onMessage) == null ? void 0 : _a6.call(handlers, envelope);
      };
      const handleClose = (event) => {
        var _a6;
        socket.removeEventListener("message", handleMessage);
        socket.removeEventListener("close", handleClose);
        socket.removeEventListener("error", handleError);
        if (this.socket === socket) {
          this.socket = null;
        }
        if (!resolved) {
          resolved = true;
          const reasonText = event.reason ? `, reason ${event.reason}` : "";
          reject(
            new Error(
              `WebSocket closed before establishing session (code ${event.code}${reasonText})`
            )
          );
          return;
        }
        (_a6 = handlers.onDisconnect) == null ? void 0 : _a6.call(handlers, event.code, event.reason);
        if (this.handlers === handlers) {
          this.handlers = null;
        }
      };
      const handleError = (event) => {
        var _a6;
        const error = event instanceof ErrorEvent && event.error instanceof Error ? event.error : new Error("WebSocket connection error.");
        if (!resolved) {
          resolved = true;
          socket.removeEventListener("open", handleOpen);
          socket.removeEventListener("message", handleMessage);
          socket.removeEventListener("close", handleClose);
          socket.removeEventListener("error", handleError);
          if (this.socket === socket) {
            this.socket = null;
          }
          reject(error);
          return;
        }
        (_a6 = handlers.onError) == null ? void 0 : _a6.call(handlers, error);
      };
      socket.addEventListener("open", handleOpen);
      socket.addEventListener("message", handleMessage);
      socket.addEventListener("close", handleClose);
      socket.addEventListener("error", handleError);
    });
  }
  async disconnect() {
    const socket = this.socket;
    if (!socket) {
      this.handlers = null;
      return;
    }
    await new Promise((resolve) => {
      const handleClose = () => {
        socket.removeEventListener("close", handleClose);
        resolve();
      };
      if (socket.readyState === WebSocket.CLOSED) {
        resolve();
        return;
      }
      socket.addEventListener("close", handleClose);
      socket.close();
    });
    if (this.socket === socket) {
      this.socket = null;
    }
    this.handlers = null;
  }
  send(data) {
    const socket = this.socket;
    if (!socket || socket.readyState !== WebSocket.OPEN) {
      throw new Error("Cannot send message: WebSocket is not connected.");
    }
    const payload = typeof data === "string" ? data : JSON.stringify(data);
    socket.send(payload);
  }
  createWebSocketUrl(playerId) {
    const { websocketUrl } = this.configuration;
    const url = websocketUrl.startsWith("ws:") || websocketUrl.startsWith("wss:") ? new URL(websocketUrl) : new URL(websocketUrl, window.location.origin);
    if (url.protocol === "http:" || url.protocol === "https:") {
      url.protocol = url.protocol === "https:" ? "wss:" : "ws:";
    }
    url.searchParams.set("id", playerId);
    return url.toString();
  }
};

// client/main.ts
var HEALTH_CHECK_URL = "/health";
var JOIN_URL = "/join";
var WEBSOCKET_URL = "/ws";
var HEARTBEAT_INTERVAL_MS = 2e3;
var PROTOCOL_VERSION = 1;
var GameClientApp = class extends i4 {
  constructor() {
    super();
    __publicField(this, "clockInterval");
    __publicField(this, "networkClient");
    __publicField(this, "hasLoggedInitialNetworkMessage");
    __publicField(this, "suppressNextDisconnectLog");
    __publicField(this, "playerId");
    this.clockInterval = void 0;
    this.networkClient = null;
    this.hasLoggedInitialNetworkMessage = false;
    this.suppressNextDisconnectLog = false;
    this.healthStatus = "Checking\u2026";
    this.serverTime = "--";
    this.heartbeat = "--";
    this.logs = [];
    this.activeTab = "telemetry";
    this.playerId = null;
    this.addLog("Booting client\u2026");
  }
  createRenderRoot() {
    return this;
  }
  connectedCallback() {
    super.connectedCallback();
    this.updateServerTime();
    void this.fetchHealth();
    void this.joinWorld();
    this.clockInterval = window.setInterval(() => {
      this.updateServerTime();
    }, 1e3);
  }
  disconnectedCallback() {
    super.disconnectedCallback();
    if (this.clockInterval) {
      window.clearInterval(this.clockInterval);
      this.clockInterval = void 0;
    }
    if (this.networkClient) {
      this.suppressNextDisconnectLog = true;
      const disconnectPromise = this.networkClient.disconnect();
      void Promise.resolve(disconnectPromise).finally(() => {
        this.suppressNextDisconnectLog = false;
      });
      this.networkClient = null;
    }
    this.hasLoggedInitialNetworkMessage = false;
  }
  addLog(message) {
    const entry = {
      timestamp: (/* @__PURE__ */ new Date()).toLocaleTimeString(),
      message
    };
    this.logs = [entry, ...this.logs].slice(0, 50);
  }
  async fetchHealth() {
    this.healthStatus = "Checking\u2026";
    try {
      const response = await fetch(HEALTH_CHECK_URL, { cache: "no-cache" });
      if (!response.ok) {
        throw new Error(`health check failed with ${response.status}`);
      }
      const text = (await response.text()).trim();
      const status = text || "ok";
      this.healthStatus = status;
      this.addLog(`Health check succeeded: ${status}`);
    } catch (error) {
      const message = error instanceof Error ? error.message : String(error);
      this.healthStatus = "offline";
      this.addLog(`Health check failed: ${message}`);
    }
  }
  updateServerTime() {
    const now = /* @__PURE__ */ new Date();
    this.serverTime = now.toLocaleTimeString();
  }
  handleRefreshRequested() {
    void this.fetchHealth();
  }
  async joinWorld() {
    this.addLog("Joining world\u2026");
    this.heartbeat = "Connecting\u2026";
    this.hasLoggedInitialNetworkMessage = false;
    if (this.networkClient) {
      this.suppressNextDisconnectLog = true;
      await this.networkClient.disconnect();
      this.suppressNextDisconnectLog = false;
    }
    const networkClient = new WebSocketNetworkClient({
      joinUrl: JOIN_URL,
      websocketUrl: WEBSOCKET_URL,
      heartbeatIntervalMs: HEARTBEAT_INTERVAL_MS,
      protocolVersion: PROTOCOL_VERSION
    });
    this.networkClient = networkClient;
    try {
      const joinResponse = await networkClient.join();
      setEffectCatalog(joinResponse.effectCatalog);
      this.playerId = joinResponse.id;
      this.addLog(`Joined world as ${joinResponse.id}.`);
      const catalogSize = Object.keys(joinResponse.effectCatalog).length;
      this.addLog(`Received ${catalogSize} effect catalog entries.`);
      await networkClient.connect({
        onJoin: () => {
          this.addLog("Connected to world stream.");
          this.heartbeat = "Connected";
        },
        onMessage: (message) => {
          this.handleNetworkMessage(message);
        },
        onDisconnect: (code, reason) => {
          const shouldLog = !this.suppressNextDisconnectLog;
          this.suppressNextDisconnectLog = false;
          if (shouldLog) {
            const reasonText = reason ? ` (${reason})` : "";
            this.addLog(`Disconnected from server${reasonText}.`);
          }
          if (this.networkClient === networkClient) {
            this.networkClient = null;
            this.playerId = null;
          }
          this.hasLoggedInitialNetworkMessage = false;
          const displayStatus = code === 1e3 ? "Disconnected" : `Closed (${code != null ? code : "--"})`;
          this.heartbeat = displayStatus;
        },
        onError: (error) => {
          this.addLog(`Network error: ${error.message}`);
        }
      });
    } catch (error) {
      const message = error instanceof Error ? error.message : String(error);
      this.playerId = null;
      this.heartbeat = "offline";
      this.networkClient = null;
      this.suppressNextDisconnectLog = false;
      this.addLog(`Failed to join world: ${message}`);
    }
  }
  handleNetworkMessage(message) {
    const timestamp = new Date(message.receivedAt).toLocaleTimeString();
    this.heartbeat = `${message.type || "unknown"} @ ${timestamp}`;
    if (!this.hasLoggedInitialNetworkMessage) {
      this.hasLoggedInitialNetworkMessage = true;
      this.addLog(`Receiving ${message.type || "unknown"} messages from server.`);
    }
  }
  handleTabChange(event) {
    this.activeTab = event.detail;
  }
  handleWorldReset(event) {
    const { seed } = event.detail;
    const seedMessage = seed ? `with seed ${seed}` : "with random seed";
    this.addLog(`World reset requested ${seedMessage}.`);
  }
  render() {
    var _a6;
    return x`
      <app-shell
        heading="Mine &amp; Die"
        subtitle="Multiplayer sandbox in active development."
        .healthStatus=${this.healthStatus}
        .logs=${this.logs}
        .serverTime=${this.serverTime}
        .heartbeat=${this.heartbeat}
        .activeTab=${this.activeTab}
        @refresh-requested=${this.handleRefreshRequested}
        @tab-change=${this.handleTabChange}
        @world-reset-requested=${this.handleWorldReset}
      ></app-shell>
      <hud-network
        .serverTime=${this.serverTime}
        .heartbeat=${this.heartbeat}
        .playerId=${(_a6 = this.playerId) != null ? _a6 : ""}
      ></hud-network>
    `;
  }
};
__publicField(GameClientApp, "properties", {
  healthStatus: { state: true },
  serverTime: { state: true },
  heartbeat: { state: true },
  logs: { state: true },
  activeTab: { state: true },
  playerId: { state: true }
});
var AppShell = class extends i4 {
  constructor() {
    super();
    __publicField(this, "heading");
    __publicField(this, "subtitle");
    __publicField(this, "healthStatus");
    __publicField(this, "logs");
    __publicField(this, "serverTime");
    __publicField(this, "heartbeat");
    __publicField(this, "activeTab");
    this.heading = "";
    this.subtitle = "";
    this.healthStatus = "--";
    this.logs = [];
    this.serverTime = "--";
    this.heartbeat = "--";
    this.activeTab = "telemetry";
  }
  createRenderRoot() {
    return this;
  }
  handleRefreshClick() {
    this.dispatchEvent(
      new CustomEvent("refresh-requested", {
        bubbles: true,
        composed: true
      })
    );
  }
  render() {
    return x`
      <main class="page">
        <header class="page-header">
          <div>
            <h1>${this.heading}</h1>
            <p class="page-header__subtitle">${this.subtitle}</p>
          </div>
          <div class="page-header__controls">
            <button
              type="button"
              class="interface-tabs__tab interface-tabs__tab--active"
              @click=${this.handleRefreshClick}
            >
              Refresh status
            </button>
            <span class="hud-network__item">${this.healthStatus}</span>
          </div>
        </header>
        <game-canvas
          .activeTab=${this.activeTab}
          .logs=${this.logs}
          .healthStatus=${this.healthStatus}
          .serverTime=${this.serverTime}
          .heartbeat=${this.heartbeat}
        ></game-canvas>
      </main>
    `;
  }
};
__publicField(AppShell, "properties", {
  heading: { type: String },
  subtitle: { type: String },
  healthStatus: { type: String },
  logs: { attribute: false },
  serverTime: { type: String },
  heartbeat: { type: String },
  activeTab: { attribute: false }
});
var GameCanvas = class extends i4 {
  constructor() {
    super();
    __publicField(this, "canvasElement", null);
    __publicField(this, "activeTab");
    __publicField(this, "logs");
    __publicField(this, "healthStatus");
    __publicField(this, "serverTime");
    __publicField(this, "heartbeat");
    this.activeTab = "telemetry";
    this.logs = [];
    this.healthStatus = "--";
    this.serverTime = "--";
    this.heartbeat = "--";
  }
  createRenderRoot() {
    return this;
  }
  firstUpdated() {
    this.canvasElement = this.querySelector("canvas");
    if (this.canvasElement) {
      this.drawBootScreen(this.canvasElement);
    }
  }
  drawBootScreen(canvas) {
    const context = canvas.getContext("2d");
    if (!context) {
      return;
    }
    const { width, height } = canvas;
    const gradient = context.createLinearGradient(0, 0, width, height);
    gradient.addColorStop(0, "#0f172a");
    gradient.addColorStop(1, "#1e293b");
    context.fillStyle = gradient;
    context.fillRect(0, 0, width, height);
    context.fillStyle = "#38bdf8";
    context.font = "24px 'Segoe UI', sans-serif";
    context.textAlign = "center";
    context.fillText("Mine & Die", width / 2, height / 2);
  }
  render() {
    return x`
      <section class="play-area">
        <div class="play-area__main">
          <canvas width="800" height="600" aria-label="Game viewport"></canvas>
        </div>
        <tabs-nav .activeTab=${this.activeTab}></tabs-nav>
        <panel-viewport
          .activeTab=${this.activeTab}
          .logs=${this.logs}
          .healthStatus=${this.healthStatus}
          .serverTime=${this.serverTime}
          .heartbeat=${this.heartbeat}
        ></panel-viewport>
      </section>
    `;
  }
};
__publicField(GameCanvas, "properties", {
  activeTab: { attribute: false },
  logs: { attribute: false },
  healthStatus: { type: String },
  serverTime: { type: String },
  heartbeat: { type: String }
});
var TabsNav = class extends i4 {
  constructor() {
    super();
    __publicField(this, "activeTab");
    this.activeTab = "telemetry";
  }
  createRenderRoot() {
    return this;
  }
  selectTab(tab) {
    if (this.activeTab === tab) {
      return;
    }
    this.dispatchEvent(
      new CustomEvent("tab-change", {
        detail: tab,
        bubbles: true,
        composed: true
      })
    );
  }
  render() {
    const tabs = [
      { id: "telemetry", label: "Telemetry" },
      { id: "world", label: "World" },
      { id: "inventory", label: "Inventory" }
    ];
    return x`
      <nav class="tabs-nav" aria-label="Client panels">
        <div class="interface-tabs__list">
          ${tabs.map((tab) => {
      const classes = e5({
        "interface-tabs__tab": true,
        "interface-tabs__tab--active": this.activeTab === tab.id
      });
      return x`
              <button
                type="button"
                class=${classes}
                @click=${() => {
        this.selectTab(tab.id);
      }}
              >
                ${tab.label}
              </button>
            `;
    })}
        </div>
      </nav>
    `;
  }
};
__publicField(TabsNav, "properties", {
  activeTab: { attribute: false }
});
var PanelViewport = class extends i4 {
  constructor() {
    super();
    __publicField(this, "activeTab");
    __publicField(this, "logs");
    __publicField(this, "healthStatus");
    __publicField(this, "serverTime");
    __publicField(this, "heartbeat");
    this.activeTab = "telemetry";
    this.logs = [];
    this.healthStatus = "--";
    this.serverTime = "--";
    this.heartbeat = "--";
  }
  createRenderRoot() {
    return this;
  }
  render() {
    return x`
      <section class="panel-viewport">
        <debug-panel
          .healthStatus=${this.healthStatus}
          .serverTime=${this.serverTime}
          .heartbeat=${this.heartbeat}
          .logs=${this.logs}
          ?hidden=${this.activeTab !== "telemetry"}
        ></debug-panel>
        <world-controls ?hidden=${this.activeTab !== "world"}></world-controls>
        <inventory-panel ?hidden=${this.activeTab !== "inventory"}></inventory-panel>
      </section>
    `;
  }
};
__publicField(PanelViewport, "properties", {
  activeTab: { attribute: false },
  logs: { attribute: false },
  healthStatus: { type: String },
  serverTime: { type: String },
  heartbeat: { type: String }
});
var DebugPanel = class extends i4 {
  constructor() {
    super();
    __publicField(this, "healthStatus");
    __publicField(this, "serverTime");
    __publicField(this, "heartbeat");
    __publicField(this, "logs");
    this.healthStatus = "--";
    this.serverTime = "--";
    this.heartbeat = "--";
    this.logs = [];
  }
  createRenderRoot() {
    return this;
  }
  render() {
    const logText = this.logs.map((log) => `[${log.timestamp}] ${log.message}`).join("\n");
    return x`
      <article class="debug-panel">
        <header class="debug-panel__header">
          <div class="debug-panel__heading">
            <h2 class="debug-panel__title">Telemetry</h2>
            <p class="debug-panel__subtitle">
              Live diagnostics from the connected client instance.
            </p>
          </div>
          <button class="debug-panel__toggle" type="button">Collapse</button>
        </header>
        <div class="debug-panel__body">
          <div class="debug-panel__summary">
            <div class="debug-panel__status">
              <span class="debug-panel__status-label">Client health</span>
              <span class="debug-panel__status-text">${this.healthStatus}</span>
            </div>
            <div class="debug-panel__metrics">
              <div class="debug-metric">
                <span class="debug-metric__label">Server time</span>
                <span class="debug-metric__value">${this.serverTime}</span>
              </div>
              <div class="debug-metric">
                <span class="debug-metric__label">Heartbeat RTT</span>
                <span class="debug-metric__value">${this.heartbeat}</span>
              </div>
            </div>
          </div>
          <section>
            <h3 class="sr-only">Client console output</h3>
            <pre class="console-output">${logText || "Booting client\u2026"}</pre>
          </section>
        </div>
      </article>
    `;
  }
};
__publicField(DebugPanel, "properties", {
  healthStatus: { type: String },
  serverTime: { type: String },
  heartbeat: { type: String },
  logs: { attribute: false }
});
var WorldControls = class extends i4 {
  createRenderRoot() {
    return this;
  }
  handleSubmit(event) {
    var _a6;
    event.preventDefault();
    const form = event.currentTarget;
    if (!form) {
      return;
    }
    const formData = new FormData(form);
    const seed = ((_a6 = formData.get("seed")) != null ? _a6 : "").toString().trim();
    this.dispatchEvent(
      new CustomEvent("world-reset-requested", {
        detail: { seed },
        bubbles: true,
        composed: true
      })
    );
    form.reset();
  }
  render() {
    return x`
      <section class="world-controls">
        <h2 class="world-controls__title">World controls</h2>
        <form class="world-controls__form" @submit=${this.handleSubmit}>
          <label class="world-controls__label">
            World seed
            <input
              type="text"
              name="seed"
              placeholder="Leave empty for random seed"
              class="world-controls__input"
            />
          </label>
          <button type="submit" class="world-controls__submit">Reset world</button>
        </form>
      </section>
    `;
  }
};
var InventoryPanel = class extends i4 {
  constructor() {
    super();
    __publicField(this, "items");
    this.items = [
      { id: 1, name: "Stone", icon: "\u{1FAA8}", quantity: 64 },
      { id: 2, name: "Wood", icon: "\u{1FAB5}", quantity: 24 },
      { id: 3, name: "Crystal", icon: "\u{1F48E}", quantity: 3 },
      { id: 4, name: "Fiber", icon: "\u{1F9F5}", quantity: 12 },
      { id: 5, name: "Essence", icon: "\u2728", quantity: 1 }
    ];
  }
  createRenderRoot() {
    return this;
  }
  render() {
    return x`
      <section class="inventory-panel">
        <h2>Inventory</h2>
        <div class="inventory-grid">
          ${this.items.map((item) => {
      return x`
              <div class="inventory-slot" role="listitem">
                <span class="inventory-item-icon" aria-hidden="true">${item.icon}</span>
                <span class="inventory-item-name">${item.name}</span>
                <span class="inventory-item-quantity">x${item.quantity}</span>
              </div>
            `;
    })}
        </div>
      </section>
    `;
  }
};
__publicField(InventoryPanel, "properties", {
  items: { attribute: false }
});
var HudNetwork = class extends i4 {
  constructor() {
    super();
    __publicField(this, "serverTime");
    __publicField(this, "heartbeat");
    __publicField(this, "playerId");
    this.serverTime = "--";
    this.heartbeat = "--";
    this.playerId = "";
  }
  createRenderRoot() {
    return this;
  }
  render() {
    return x`
      <div class="hud-network">
        <span class="hud-network__item">Server time: ${this.serverTime}</span>
        <span class="hud-network__item">Heartbeat: ${this.heartbeat}</span>
        <span class="hud-network__item">
          Player: ${this.playerId ? this.playerId : "\u2014"}
        </span>
      </div>
    `;
  }
};
__publicField(HudNetwork, "properties", {
  serverTime: { type: String },
  heartbeat: { type: String },
  playerId: { type: String }
});
customElements.define("game-client-app", GameClientApp);
customElements.define("app-shell", AppShell);
customElements.define("game-canvas", GameCanvas);
customElements.define("tabs-nav", TabsNav);
customElements.define("panel-viewport", PanelViewport);
customElements.define("debug-panel", DebugPanel);
customElements.define("world-controls", WorldControls);
customElements.define("inventory-panel", InventoryPanel);
customElements.define("hud-network", HudNetwork);
/*! Bundled license information:

@lit/reactive-element/css-tag.js:
  (**
   * @license
   * Copyright 2019 Google LLC
   * SPDX-License-Identifier: BSD-3-Clause
   *)

@lit/reactive-element/reactive-element.js:
lit-html/lit-html.js:
lit-element/lit-element.js:
lit-html/directive.js:
  (**
   * @license
   * Copyright 2017 Google LLC
   * SPDX-License-Identifier: BSD-3-Clause
   *)

lit-html/is-server.js:
  (**
   * @license
   * Copyright 2022 Google LLC
   * SPDX-License-Identifier: BSD-3-Clause
   *)

lit-html/directives/class-map.js:
  (**
   * @license
   * Copyright 2018 Google LLC
   * SPDX-License-Identifier: BSD-3-Clause
   *)
*/
//# sourceMappingURL=main.js.map
