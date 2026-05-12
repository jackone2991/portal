@extends('index')
    @section('head-meta')
        <meta charset="UTF-8">
        <meta name="csrf-token" content="{{ csrf_token() }}">
        <meta http-equiv="Content-Type" content="text/html; charset=UTF-8">
        <!-- Required meta tags always come first -->
        <meta name="viewport" content="width=device-width, initial-scale=1">
        <meta http-equiv="x-ua-compatible" content="ie=edge">
    @endsection
    @section('head-title')
        Newfeeds
    @endsection
    @section('head-fonts')
        @include('v1.components.head.font_public')
    @endsection
    @section('head-styles')
        @include('v1.components.head.css')
    @endsection
    @section('head-script')
        @include('v1.components.head.js')
    @endsection
    @section('body-header')
        @include('v1.partials.hellopreloader')
        <!-- Fixed Sidebar Left -->
        @include('v1.components.menu.sidebarLeft')
        <!-- Fixed Sidebar Left -->
        <!-- ... start Fixed Sidebar Right-Responsive -->
        @include('v1.components.menu.sidebarRight')
        <!-- ... end Fixed Sidebar Right-Responsive -->
        <!-- Header-BP -->
        @include('v1.components.menu.sidebarCenter')
        <!-- ... end Header-BP -->
        <!-- Responsive Header-BP -->
        @include('v1.components.menu.sidebarCenterResponsive')
        <!-- ... end Responsive Header-BP -->
        <div class="header-spacer"></div>
    @endsection
    @section('popup')
        @include('v1.partials.goToTop')
        @include('v1.components.popup.updateHeaderPhoto')
        @include('v1.components.popup.choseFromMyPhoto')
        @include('v1.components.popup.chatResponsive')
    @endsection
    @section('body-scripts')
        @include('v1.components.footers.js')
        @include('v1.components.footers.svg')
        @include('v1.components.footers.ico')
    @endsection
