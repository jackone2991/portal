<!DOCTYPE html>
<!-- saved from url=(0054)./03-Newsfeed.html -->
<html lang="en">
    <head>
        <meta charset="UTF-8">
        <meta name="csrf-token" content="{{ csrf_token() }}">
        {{-- Part: all meta-related contents --}}
        @yield('head-meta')
        {{-- Part: site title with default value in parent --}}
        @section('head-title')
            <title>Lam Dep Trai</title>
        @show
        @section('head-fonts')
        @show
        @section('head-styles')
            @include('v1.components.head.public')
        @show
        @section('head-script')
        @show
        @section('head-extra')
        @show

    </head>
        {{-- Part: something at start of body --}}
    <body class="page-has-left-panels page-has-right-panels"><span id="warning-container"><i data-reactroot=""></i></span>
    {{-- <body> --}}
        @yield('body-start')
        {{-- Part: header of body --}}
        @section('body-header')
            {{-- Part: navigation bar --}}
            {{--  @include('partials.navbar') --}}

        @show

        {{-- Part: create main content of the page --}}
        @section('body-content')
        {{-- @show --}}

        {{-- Part: footer --}}
        @section('body-footer')
            {{-- Part: footer is probably shared across many pages --}}
            {{--  @include('partials.footer') --}}
        @show
        {{-- Part: load scripts --}}
        @section('body-scripts')
            {{-- Part: footer is probably shared across many pages --}}
            @include('v1.components.footers.public')
            {{--  @include('partials.footer') --}}

        @show
        {{-- Part: something else to do --}}
        @yield('body-others')
        {{-- Part: finalize stuffs if there is --}}
        @yield('body-end')

    </body>
</html>
