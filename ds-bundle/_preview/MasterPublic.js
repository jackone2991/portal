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

  // .design-sync/previews/MasterPublic.tsx
  var MasterPublic_exports = {};
  __export(MasterPublic_exports, {
    AuthCard: () => AuthCard
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

  // .design-sync/previews/MasterPublic.tsx
  var import_jsx_runtime = __toESM(require_react_shim());
  var AuthCard = () => /* @__PURE__ */ (0, import_jsx_runtime.jsx)(ds_exports.MasterPublic, { children: /* @__PURE__ */ (0, import_jsx_runtime.jsx)(
    "div",
    {
      style: {
        minHeight: 560,
        display: "grid",
        placeItems: "center",
        padding: 24,
        background: "var(--tpl-bg, #f5f6fa)"
      },
      children: /* @__PURE__ */ (0, import_jsx_runtime.jsxs)(
        "div",
        {
          style: {
            width: 360,
            background: "#fff",
            borderRadius: 16,
            border: "1px solid var(--tpl-border, #e6ecf5)",
            boxShadow: "0 12px 40px rgba(63,66,87,0.10)",
            padding: 32
          },
          children: [
            /* @__PURE__ */ (0, import_jsx_runtime.jsx)(
              "div",
              {
                style: {
                  width: 48,
                  height: 48,
                  borderRadius: 12,
                  marginBottom: 20,
                  background: "linear-gradient(135deg, var(--tpl-accent, #ff5e3a), var(--tpl-accent-2, #ff763a))"
                }
              }
            ),
            /* @__PURE__ */ (0, import_jsx_runtime.jsx)(
              "h1",
              {
                style: {
                  margin: 0,
                  fontSize: 22,
                  fontWeight: 700,
                  color: "var(--tpl-heading, #3f4257)"
                },
                children: "Welcome back"
              }
            ),
            /* @__PURE__ */ (0, import_jsx_runtime.jsx)("p", { style: { margin: "6px 0 24px", fontSize: 14, color: "var(--tpl-muted, #888da8)" }, children: "Sign in to your Portal account" }),
            [
              { label: "Email", value: "ada@portal.dev" },
              { label: "Password", value: "••••••••••" }
            ].map((f) => /* @__PURE__ */ (0, import_jsx_runtime.jsxs)("div", { style: { marginBottom: 16 }, children: [
              /* @__PURE__ */ (0, import_jsx_runtime.jsx)(
                "label",
                {
                  style: {
                    display: "block",
                    fontSize: 12,
                    fontWeight: 600,
                    marginBottom: 6,
                    color: "var(--tpl-text, #515365)"
                  },
                  children: f.label
                }
              ),
              /* @__PURE__ */ (0, import_jsx_runtime.jsx)(
                "div",
                {
                  style: {
                    height: 42,
                    display: "flex",
                    alignItems: "center",
                    padding: "0 12px",
                    fontSize: 14,
                    borderRadius: 10,
                    color: "var(--tpl-text, #515365)",
                    border: "1px solid var(--tpl-border, #e6ecf5)",
                    background: "var(--tpl-surface-2, #f6f7fb)"
                  },
                  children: f.value
                }
              )
            ] }, f.label)),
            /* @__PURE__ */ (0, import_jsx_runtime.jsx)(
              "button",
              {
                type: "button",
                style: {
                  width: "100%",
                  height: 44,
                  marginTop: 8,
                  border: 0,
                  borderRadius: 10,
                  fontSize: 14,
                  fontWeight: 700,
                  color: "#fff",
                  cursor: "pointer",
                  background: "linear-gradient(135deg, var(--tpl-accent, #ff5e3a), var(--tpl-accent-2, #ff763a))"
                },
                children: "Sign in"
              }
            ),
            /* @__PURE__ */ (0, import_jsx_runtime.jsxs)(
              "p",
              {
                style: {
                  margin: "18px 0 0",
                  textAlign: "center",
                  fontSize: 13,
                  color: "var(--tpl-muted, #888da8)"
                },
                children: [
                  "New here?",
                  " ",
                  /* @__PURE__ */ (0, import_jsx_runtime.jsx)("span", { style: { fontWeight: 600, color: "var(--tpl-accent, #ff5e3a)" }, children: "Create an account" })
                ]
              }
            )
          ]
        }
      )
    }
  ) });
  return __toCommonJS(MasterPublic_exports);
})();
