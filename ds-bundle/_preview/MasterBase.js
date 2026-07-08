var __dsPreview = (() => {
  var __create = Object.create;
  var __defProp = Object.defineProperty;
  var __getOwnPropDesc = Object.getOwnPropertyDescriptor;
  var __getOwnPropNames = Object.getOwnPropertyNames;
  var __getProtoOf = Object.getPrototypeOf;
  var __hasOwnProp = Object.prototype.hasOwnProperty;
  var __esm = (fn, res, err) => function __init() {
    if (err) throw err[0];
    try {
      return fn && (res = (0, fn[__getOwnPropNames(fn)[0]])(fn = 0)), res;
    } catch (e) {
      throw err = [e], e;
    }
  };
  var __commonJS = (cb, mod) => function __require() {
    try {
      return mod || (0, cb[__getOwnPropNames(cb)[0]])((mod = { exports: {} }).exports, mod), mod.exports;
    } catch (e) {
      throw mod = 0, e;
    }
  };
  var __export = (target, all) => {
    for (var name in all)
      __defProp(target, name, { get: all[name], enumerable: true });
  };
  var __copyProps = (to, from, except, desc) => {
    if (from && typeof from === "object" || typeof from === "function") {
      for (let key of __getOwnPropNames(from))
        if (!__hasOwnProp.call(to, key) && key !== except)
          __defProp(to, key, { get: () => from[key], enumerable: !(desc = __getOwnPropDesc(from, key)) || desc.enumerable });
    }
    return to;
  };
  var __reExport = (target, mod, secondTarget) => (__copyProps(target, mod, "default"), secondTarget && __copyProps(secondTarget, mod, "default"));
  var __toESM = (mod, isNodeMode, target) => (target = mod != null ? __create(__getProtoOf(mod)) : {}, __copyProps(
    // If the importer is in node compatibility mode or this is not an ESM
    // file that has been converted to a CommonJS file using a Babel-
    // compatible transform (i.e. "__esModule" has not been set), then set
    // "default" to the CommonJS "module.exports" for node compatibility.
    isNodeMode || !mod || !mod.__esModule ? __defProp(target, "default", { value: mod, enumerable: true }) : target,
    mod
  ));
  var __toCommonJS = (mod) => __copyProps(__defProp({}, "__esModule", { value: true }), mod);

  // <define:import.meta.env>
  var init_define_import_meta_env = __esm({
    "<define:import.meta.env>"() {
    }
  });

  // ds-raw:__ds_raw__
  var require_ds_raw = __commonJS({
    "ds-raw:__ds_raw__"(exports, module) {
      init_define_import_meta_env();
      module.exports = window.PortalUI;
    }
  });

  // shim:react-shim
  var require_react_shim = __commonJS({
    "shim:react-shim"(exports, module) {
      init_define_import_meta_env();
      var R = window.React;
      function np(p, k) {
        var o = {};
        for (var x in p) if (x !== "children") o[x] = p[x];
        if (k !== void 0) o.key = k;
        return o;
      }
      function jsx2(t, p, k) {
        var c = p && p.children;
        return c === void 0 ? R.createElement(t, np(p, k)) : R.createElement(t, np(p, k), c);
      }
      function jsxs2(t, p, k) {
        return R.createElement.apply(R, [t, np(p, k)].concat(p.children));
      }
      module.exports = R;
      module.exports.jsx = jsx2;
      module.exports.jsxs = jsxs2;
      module.exports.jsxDEV = function(t, p, k, s) {
        return (s ? jsxs2 : jsx2)(t, p, k);
      };
      module.exports.Fragment = R.Fragment;
    }
  });

  // .design-sync/previews/MasterBase.tsx
  var MasterBase_exports = {};
  __export(MasterBase_exports, {
    App: () => App
  });
  init_define_import_meta_env();

  // ds-shim:ds
  var ds_exports = {};
  __export(ds_exports, {
    default: () => ds_default
  });
  init_define_import_meta_env();
  __reExport(ds_exports, __toESM(require_ds_raw()));
  var g = window.PortalUI;
  var ds_default = "default" in g ? g.default : g;

  // .design-sync/previews/MasterBase.tsx
  var import_jsx_runtime = __toESM(require_react_shim());
  var css = `
#hellopreloader{display:none !important}
.mb-wrap{position:relative;width:850px;height:640px;overflow:hidden}
.mb-wrap header{position:absolute !important}
.mb-wrap aside{display:flex !important;position:absolute !important;height:568px !important}
.mb-wrap main{padding-left:var(--tpl-sidebar-w) !important;padding-right:var(--tpl-rightbar-w) !important;min-height:640px}
`;
  var card = {
    background: "#fff",
    borderRadius: 12,
    border: "1px solid var(--tpl-border, #e6ecf5)",
    padding: 16,
    marginBottom: 14
  };
  var App = () => /* @__PURE__ */ (0, import_jsx_runtime.jsxs)("div", { className: "mb-wrap", children: [
    /* @__PURE__ */ (0, import_jsx_runtime.jsx)("style", { children: css }),
    /* @__PURE__ */ (0, import_jsx_runtime.jsx)(ds_exports.MasterBase, { children: /* @__PURE__ */ (0, import_jsx_runtime.jsxs)("div", { style: { paddingTop: 4 }, children: [
      /* @__PURE__ */ (0, import_jsx_runtime.jsx)(
        "h1",
        {
          style: {
            margin: "0 0 4px",
            fontSize: 20,
            fontWeight: 700,
            color: "var(--tpl-heading, #3f4257)"
          },
          children: "Newsfeed"
        }
      ),
      /* @__PURE__ */ (0, import_jsx_runtime.jsx)("p", { style: { margin: "0 0 16px", fontSize: 13, color: "var(--tpl-muted, #888da8)" }, children: "Latest from people you follow" }),
      /* @__PURE__ */ (0, import_jsx_runtime.jsxs)("div", { style: card, children: [
        /* @__PURE__ */ (0, import_jsx_runtime.jsxs)("div", { style: { display: "flex", alignItems: "center", gap: 10, marginBottom: 10 }, children: [
          /* @__PURE__ */ (0, import_jsx_runtime.jsx)(
            "div",
            {
              style: {
                width: 38,
                height: 38,
                borderRadius: "50%",
                background: "linear-gradient(135deg, var(--tpl-accent, #ff5e3a), var(--tpl-accent-2, #ff763a))"
              }
            }
          ),
          /* @__PURE__ */ (0, import_jsx_runtime.jsxs)("div", { children: [
            /* @__PURE__ */ (0, import_jsx_runtime.jsx)("div", { style: { fontSize: 13, fontWeight: 600, color: "var(--tpl-heading, #3f4257)" }, children: "Carol Summers" }),
            /* @__PURE__ */ (0, import_jsx_runtime.jsx)("div", { style: { fontSize: 11, color: "var(--tpl-muted, #888da8)" }, children: "2 hours ago" })
          ] })
        ] }),
        /* @__PURE__ */ (0, import_jsx_runtime.jsx)("p", { style: { margin: 0, fontSize: 13, lineHeight: 1.6, color: "var(--tpl-text, #515365)" }, children: "Just uploaded a new travel vlog — transcoding to HLS now. Playback in a minute!" }),
        /* @__PURE__ */ (0, import_jsx_runtime.jsx)(
          "div",
          {
            style: {
              height: 120,
              marginTop: 12,
              borderRadius: 10,
              background: "var(--tpl-surface-2, #f6f7fb)"
            }
          }
        )
      ] }),
      /* @__PURE__ */ (0, import_jsx_runtime.jsx)("div", { style: card, children: /* @__PURE__ */ (0, import_jsx_runtime.jsxs)("div", { style: { display: "flex", alignItems: "center", gap: 10 }, children: [
        /* @__PURE__ */ (0, import_jsx_runtime.jsx)(
          "div",
          {
            style: {
              width: 38,
              height: 38,
              borderRadius: "50%",
              background: "var(--tpl-blue, #38a9ff)"
            }
          }
        ),
        /* @__PURE__ */ (0, import_jsx_runtime.jsxs)("p", { style: { margin: 0, fontSize: 13, color: "var(--tpl-text, #515365)" }, children: [
          /* @__PURE__ */ (0, import_jsx_runtime.jsx)("strong", { style: { color: "var(--tpl-heading, #3f4257)" }, children: "Nina Kraviz" }),
          " added 3 tracks to a playlist"
        ] })
      ] }) })
    ] }) })
  ] });
  return __toCommonJS(MasterBase_exports);
})();
