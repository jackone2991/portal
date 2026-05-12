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

    @section('body-scripts')
        @include('v1.components.footers.js')
        @include('v1.components.footers.svg')
        @include('v1.components.footers.ico')
    @endsection
