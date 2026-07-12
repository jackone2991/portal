import type { TemplateManifest } from "../types";
import { MasterBase } from "./master/MasterBase";
import { MasterPublic } from "./master/MasterPublic";
import { HomeView } from "./views/home/HomeView";
import { LoginView } from "./views/auth/LoginView";
import { RegisterView } from "./views/auth/RegisterView";
import { ComicIndexView } from "./views/library/comic/ComicIndexView";
import { NovelDetailView } from "./views/library/novel/NovelDetailView";
import { MediaIndexView } from "./views/library/media/MediaIndexView";

/**
 * Template "v1" — React/Next.js port of the Crumina "Olympus" social theme at
 * `template-main/portal/resources/views/v1/`.
 */
export const v1: TemplateManifest = {
  version: "v1",
  label: "Olympus v1",
  shells: {
    public: MasterPublic,
    app: MasterBase,
  },
  views: {
    home: HomeView,
    login: LoginView,
    register: RegisterView,
    libraryComic: ComicIndexView,
    libraryNovelDetail: NovelDetailView,
    libraryMedia: MediaIndexView,
  },
};
