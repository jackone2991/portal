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
      function jsxs(t, p, k) {
        return R.createElement.apply(R, [t, np(p, k)].concat(p.children));
      }
      module.exports = R;
      module.exports.jsx = jsx2;
      module.exports.jsxs = jsxs;
      module.exports.jsxDEV = function(t, p, k, s) {
        return (s ? jsxs : jsx2)(t, p, k);
      };
      module.exports.Fragment = R.Fragment;
    }
  });

  // .design-sync/previews/Post.tsx
  var Post_exports = {};
  __export(Post_exports, {
    TextPost: () => TextPost,
    VideoPost: () => VideoPost
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

  // .design-sync/previews/Post.tsx
  var import_jsx_runtime = __toESM(require_react_shim());
  var frame = { background: "#f5f6fa", padding: 16, maxWidth: 640 };
  var TextPost = () => /* @__PURE__ */ (0, import_jsx_runtime.jsx)("div", { "data-template": "v1", style: frame, children: /* @__PURE__ */ (0, import_jsx_runtime.jsx)(
    ds_exports.Post,
    {
      author: "Marina Valentine",
      action: "shared a thought",
      time: "18 minutes ago",
      text: "Just wrapped the final colour pass on the summer short film. Six months of night shoots and it finally looks the way it sounded in my head. Screening the first cut for the crew on Friday.",
      likes: 128,
      likedBy: ["Diego Morales", "Priya Anand", "Anselm Richter"],
      comments: 24,
      shares: 7,
      liked: true
    }
  ) });
  var VideoPost = () => /* @__PURE__ */ (0, import_jsx_runtime.jsx)("div", { "data-template": "v1", style: frame, children: /* @__PURE__ */ (0, import_jsx_runtime.jsx)(
    ds_exports.Post,
    {
      author: "Diego Morales",
      action: "posted a video",
      time: "2 hours ago",
      text: "New behind-the-scenes reel is live — camera tests, lens breakdowns, and the gimbal rig that saved the rooftop chase sequence.",
      media: {
        type: "video",
        title: "Behind the Lens: Shooting the Rooftop Chase",
        desc: "A twelve-minute walkthrough of the anamorphic setup and the lighting plan for the night exterior.",
        source: "PORTAL STUDIO · 12:04"
      },
      likes: 342,
      likedBy: ["Marina Valentine", "Nadia Okonkwo", "Priya Anand", "Anselm Richter"],
      comments: 58,
      shares: 19
    }
  ) });
  return __toCommonJS(Post_exports);
})();
