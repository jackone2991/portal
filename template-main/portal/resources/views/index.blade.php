<html lang="en">
    <head>
        @yield('head-meta')
        <title>@yield('head-title', 'ahihi')</title>
        @yield('head-fonts')
        @yield('head-styles')
        @yield('head-script')
    </head>
    <!-- <body class="page-has-left-panels page-has-right-panels"> -->
    
        @yield('body-start')
        
        @yield('body-header')
        @yield('body-popup')
        @yield('body-content')
        @yield('body-footer')
        @yield('body-scripts')
        @yield('body-start')
        @yield('body-others')
        @yield('body-end')

</html>
