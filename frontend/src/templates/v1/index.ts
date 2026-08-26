import type { TemplateManifest } from "../types";
import { MasterBase } from "./master/MasterBase";
import { MasterPublic } from "./master/MasterPublic";
import { HomeView } from "./views/home/HomeView";
import { LoginView } from "./views/auth/LoginView";
import { RegisterView } from "./views/auth/RegisterView";
import { ComicIndexView } from "./views/library/comic/ComicIndexView";
import { ComicDetailView } from "./views/library/comic/ComicDetailView";
import { ComicReaderView } from "./views/library/comic/ComicReaderView";
import { NovelDetailView } from "./views/library/novel/NovelDetailView";
import { MediaIndexView } from "./views/library/media/MediaIndexView";
import { MediaDetailView } from "./views/library/media/MediaDetailView";
import { MusicIndexView } from "./views/library/music/MusicIndexView";
import { MusicDetailView } from "./views/library/music/MusicDetailView";
import { DashboardView as BankDashboardView } from "./views/bank/DashboardView";
import { TransactionsView as BankTransactionsView } from "./views/bank/TransactionsView";
import { AccountsView as BankAccountsView } from "./views/bank/AccountsView";
import { BudgetsView as BankBudgetsView } from "./views/bank/BudgetsView";
import { PeopleIndexView } from "./views/people/PeopleIndexView";
import { PersonDetailView } from "./views/people/PersonDetailView";
import { CalendarView } from "./views/calendar/CalendarView";
import { WeatherView } from "./views/weather/WeatherView";

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
    libraryComicDetail: ComicDetailView,
    libraryComicReader: ComicReaderView,
    libraryNovelDetail: NovelDetailView,
    libraryMedia: MediaIndexView,
    libraryMediaDetail: MediaDetailView,
    libraryMusic: MusicIndexView,
    libraryMusicDetail: MusicDetailView,
    bankDashboard: BankDashboardView,
    bankTransactions: BankTransactionsView,
    bankAccounts: BankAccountsView,
    bankBudgets: BankBudgetsView,
    peopleList: PeopleIndexView,
    peopleDetail: PersonDetailView,
    calendar: CalendarView,
    weather: WeatherView,
  },
};
