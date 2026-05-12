@extends('v1.master.master-public')
@section('body-content')
    <body class="landing-page">
        <span id="warning-container">
            <i data-reactroot=""></i>
        </span>
        <!-- Preloader -->
        <div id="hellopreloader">
            <div class="preloader">
                <svg width="45" height="45" stroke="#fff">
                    <g fill="none" fill-rule="evenodd" stroke-width="2" transform="translate(1 1)">
                        <circle cx="22" cy="22" r="6" stroke="none">
                            <animate attributeName="r" begin="1.5s" calcMode="linear" dur="3s" repeatCount="indefinite" values="6;22"></animate>
                            <animate attributeName="stroke-opacity" begin="1.5s" calcMode="linear" dur="3s" repeatCount="indefinite" values="1;0"></animate>
                            <animate attributeName="stroke-width" begin="1.5s" calcMode="linear" dur="3s" repeatCount="indefinite" values="2;0"></animate>
                        </circle>
                        <circle cx="22" cy="22" r="6" stroke="none">
                            <animate attributeName="r" begin="3s" calcMode="linear" dur="3s" repeatCount="indefinite" values="6;22"></animate>
                            <animate attributeName="stroke-opacity" begin="3s" calcMode="linear" dur="3s" repeatCount="indefinite" values="1;0"></animate>
                            <animate attributeName="stroke-width" begin="3s" calcMode="linear" dur="3s" repeatCount="indefinite" values="2;0"></animate>
                        </circle>
                        <circle cx="22" cy="22" r="8">
                            <animate attributeName="r" begin="0s" calcMode="linear" dur="1.5s" repeatCount="indefinite" values="6;1;2;3;4;5;6"></animate>
                        </circle>
                    </g>
                </svg>

                <div class="text">Loading ...</div>
            </div>
        </div>
        <!-- ... end Preloader -->
        <div class="content-bg-wrap"></div>
        <!-- Header Standard Landing  -->
    <div class="header--standard header--standard-landing" id="header--standard">
        <div class="container">
            <div class="header--standard-wrap">
                <a href="{{ route('login') }}" class="logo">
                    <div class="img-wrap">
                        <img loading="lazy" src="{{asset('v1/ico/logo.webp')}}" alt="Olympus" width="34" height="34">
                        <img loading="lazy" src="{{asset('v1/ico/logo-colored-small.webp')}}" width="34" height="34" alt="Olympus" class="logo-colored">
                    </div>
                    <div class="title-block">
                        <h6 class="logo-title">Sky Feeling</h6>
                        <div class="sub-title">SOCIAL NETWORK</div>
                    </div>
                </a>
                <a href="{{ route('login') }}" class="open-responsive-menu js-open-responsive-menu">
                    <svg class="olymp-menu-icon"><use xlink:href="#olymp-menu-icon"></use></svg>
                </a>
            </div>
        </div>
    </div>
    <!-- ... end Header Standard Landing  -->
    <div class="header-spacer--standard"></div>
    <div class="container">
        <div class="row display-flex">
            <div class="col col-xl-6 col-lg-6 col-md-12 col-sm-12 col-12">
                <div class="landing-content">
                    <h1>Let's build your world!!!</h1>
                    <p> All thing which you need to make you happy. Sky Feeling make it for you.
                    </p>
                    <a href="{{ route('login') }}" class="btn btn-md btn-border c-white">Register Now!</a>
                </div>
            </div>
            <div class="col col-xl-5 col-lg-6 col-md-12 col-sm-12 col-12">
                <!-- Login-Registration Form  -->
                <div class="registration-login-form">
                    <!-- Nav tabs -->
                    <ul class="nav nav-tabs" id="registration-form-tabs" role="tablist">
                        <li class="nav-item" role="presentation">
                            <a class="nav-link active" id="login-tab" data-bs-toggle="tab" href="#login" role="tab" aria-controls="login" aria-selected="true">
                                <svg class="olymp-login-icon"><use xlink:href="#olymp-login-icon"></use></svg>
                            </a>
                        </li>
                        <li class="nav-item" role="presentation">
                            <a class="nav-link" id="profile-tab" data-bs-toggle="tab" href="#profile" role="tab" aria-controls="profile" aria-selected="false">
                                <svg class="olymp-register-icon"><use xlink:href="#olymp-register-icon"></use></svg>
                            </a>
                        </li>
                    </ul>
                    <!-- Tab panes -->
                    <div class="tab-content" id="registration-form-tabs-content">
                        <div class="tab-pane fade " id="profile" role="tabpanel" aria-labelledby="profile-tab">
                            <div class="title h6">Register to Sky Feeling</div>
                            <form class="content">
                                <div class="row">
                                    <div class="col col-12 col-xl-6 col-lg-6 col-md-6 col-sm-12">
                                        <div class="form-group label-floating is-empty">
                                            <label class="control-label">First Name</label>
                                            <input class="form-control" placeholder="" type="text" name="first_name" id="first_name">
                                        <span class="material-input"></span></div>
                                    </div>
                                    <div class="col col-12 col-xl-6 col-lg-6 col-md-6 col-sm-12">
                                        <div class="form-group label-floating is-empty">
                                            <label class="control-label">Last Name</label>
                                            <input class="form-control" placeholder="" type="text" name="last_name" id="last_name">
                                        <span class="material-input"></span></div>
                                    </div>
                                    <div class="col col-12 col-xl-12 col-lg-12 col-md-12 col-sm-12">
                                        <div class="form-group label-floating is-empty">
                                            <label class="control-label">Your Email</label>
                                            <input class="form-control" placeholder="" type="email" name="email" id="email">
                                        <span class="material-input"></span></div>
                                        <div class="form-group label-floating is-empty">
                                            <label class="control-label">Your Password</label>
                                            <input class="form-control" placeholder="" type="password" name="password" id="password">
                                        <span class="material-input"></span></div>

                                        <div class="form-group date-time-picker label-floating">
                                            <label class="control-label">Your Birthday</label>
                                            <input name="birthday" value="10/24/1984" class="datetimepicker" id="birthday">
                                            <span class="input-group-addon">
                                                <svg class="olymp-calendar-icon"><use xlink:href="#olymp-calendar-icon"></use></svg>
                                            </span>
                                        </div>
                                        <div class="form-group label-floating is-select">
                                            <label class="control-label">Your Gender</label>
                                            <select  name="sex" class="form-select" id="sex">
                                                <option value="male">Male</option>
                                                <option value="female">Female</option>
                                                <option value="other">Other</option>
                                            </select>
                                        </div>

                                        <div class="remember">
                                            <div class="checkbox">
                                                <label>
                                                    <input name="termCond" type="checkbox" id="termCond"><span class="checkbox-material"></span>
                                                    I accept the <a href="https://html.crumina.net/html-olympus/01-LandingPage.html#">Terms and Conditions</a> of the website
                                                </label>
                                            </div>
                                        </div>
                                        <a href="javascript:void(0)"  id="btnRegister" class="btn btn-purple btn-lg full-width">Complete Registration!</a>
                                    </div>
                                </div>
                            </form>
                        </div>
                        <div class="tab-pane fade show active" id="login" role="tabpanel" aria-labelledby="login-tab">
                            <div class="title h6">Login to your Account</div>
                            <form class="content">
                                <div class="row">
                                    <div class="col col-12 col-xl-12 col-lg-12 col-md-12 col-sm-12">
                                        <div class="form-group label-floating is-empty">
                                            <label class="control-label">Your Email</label>
                                            <input class="form-control" name='emailLogin' id="emailLogin" placeholder="" type="email">
                                        <span class="material-input"></span></div>
                                        <div class="form-group label-floating is-empty">
                                            <label class="control-label">Your Password</label>
                                            <input class="form-control" name="passLogin" id= "passLogin" placeholder="" type="password">
                                        <span class="material-input"></span></div>

                                        <div class="remember">

                                            <div class="checkbox">
                                                <label>
                                                    <input name="rememberMe" id="remberMeLogin" type="checkbox"><span class="checkbox-material"></span>
                                                    Remember Me
                                                </label>
                                            </div>
                                            <a href="https://html.crumina.net/html-olympus/01-LandingPage.html#" class="forgot" data-bs-toggle="modal" data-bs-target="#restore-password">Forgot my Password</a>
                                        </div>

                                        <a href="javascript:void(0)" id="btnLogin" class="btn btn-lg btn-primary full-width">Login</a>

                                        <div class="or"></div>

                                        <a href="{{ route('facebook.login') }}" class="btn btn-lg bg-facebook full-width btn-icon-left"><svg width="16" height="16"><use xlink:href="#olymp-facebook-icon"></use></svg>Login with Facebook</a>

                                        <a href="https://html.crumina.net/html-olympus/01-LandingPage.html#" class="btn btn-lg bg-twitter full-width btn-icon-left"><svg width="16" height="16"><use xlink:href="#olymp-twitter-icon"></use></svg>Login with Twitter</a>


                                        <p>Don’t you have an account? <a href="{{route('login')}}">Register Now!</a> it’s really simple and you can start enjoing all the benefits!</p>
                                    </div>
                                </div>
                            </form>
                        </div>
                    </div>
                </div>
                <!-- ... end Login-Registration Form  -->		</div>
        </div>
    </div>
    <!-- Window-popup Restore Password -->
    <div class="modal fade" id="restore-password" tabindex="-1" role="dialog" aria-labelledby="restore-password" aria-hidden="true">
        <div class="modal-dialog window-popup restore-password-popup" role="document">
            <div class="modal-content">
                <a href="https://html.crumina.net/html-olympus/01-LandingPage.html#" class="close icon-close" data-bs-dismiss="modal" aria-label="Close">
                    <svg class="olymp-close-icon"><use xlink:href="#olymp-close-icon"></use></svg>
                </a>
                <div class="modal-header">
                    <h6 class="title">Restore your Password</h6>
                </div>
                <div class="modal-body">
                    <form method="get">
                        <p>Enter your email and click the send code button. You’ll receive a code in your email. Please use that
                            code below to change the old password for a new one.
                        </p>
                        <div class="form-group label-floating">
                            <label class="control-label">Your Email</label>
                            <input class="form-control" placeholder="" type="email" value="james-spiegel@yourmail.com">
                        <span class="material-input"></span></div>
                        <button class="btn btn-purple btn-lg full-width">Send me the Code</button>
                        <div class="form-group label-floating is-empty">
                            <label class="control-label">Enter the Code</label>
                            <input class="form-control" placeholder="" type="text" value="">
                        <span class="material-input"></span></div>
                        <div class="form-group label-floating">
                            <label class="control-label">Your New Password</label>
                            <input class="form-control" placeholder="" type="password" value="olympus">
                        <span class="material-input"></span></div>
                        <button class="btn btn-primary btn-lg full-width">Change your Password!</button>
                    </form>
                </div>
            </div>
        </div>
    </div>
    <!-- ... end Window-popup Restore Password -->


    <!-- Window Popup Main Search -->

    <div class="modal fade" id="main-popup-search" tabindex="-1" role="dialog" aria-labelledby="main-popup-search" aria-hidden="true">
        <div class="modal-dialog modal-dialog-centered window-popup main-popup-search" role="document">
            <div class="modal-content">
                <a href="https://html.crumina.net/html-olympus/01-LandingPage.html#" class="close icon-close" data-bs-dismiss="modal" aria-label="Close">
                    <svg class="olymp-close-icon"><use xlink:href="#olymp-close-icon"></use></svg>
                </a>
                <div class="modal-body">
                    <form class="form-inline search-form" method="post">
                        <div class="form-group label-floating is-empty">
                            <label class="control-label">What are you looking for?</label>
                            <input class="form-control bg-white" placeholder="" type="text" value="">
                        <span class="material-input"></span></div>

                        <button class="btn btn-purple btn-lg">Search</button>
                    </form>
                </div>
            </div>
        </div>
    </div>


<script  type="text/javascript">
    $(document).ready(function () {
        $("#btnRegister").click(function () {
            var termCond = false;
            if ($("#termCond").prop("checked") == true) {
                $.ajax({
                    method: "POST",
                    url: "{{route('api.post.register')}}",
                    dataType: "json",
                    data: {
                        "_token": "{{ csrf_token() }}",
                        first_name: $("#first_name").val(),
                        last_name: $("#last_name").val(),
                        email: $("#email").val(),
                        password: $("#password").val(),
                        birthday: $("#birthday").val(),
                        sex: $("#sex").val(),
                        termCond: termCond,
                    },
                })
                .done(function (data) {
                    window.location.href = "{{  route('login') }}";

                })
                .fail(function () {
                    alert("Da co loi xay ra.");
                });

            }else{
                alert("Please agree our terms and conditions");
            }
        });
        $("#btnLogin").click(function () {
            var remberMeLogin = false;
            if ($("#remberMeLogin").prop("checked") == true) {
                 remberMeLogin = true;
            }
            $.ajax({
                headers: {
                    'X-CSRF-TOKEN': $('meta[name="csrf-token"]').attr('content')
                },
                method: "POST",
                url: "{{route('api.post.login')}}",
                dataType: "json",
                data: {
                    _token: "{{ csrf_token() }}",
                    email: $("#emailLogin").val(),
                    password: $("#passLogin").val(),
                    remberMeLogin: remberMeLogin,
                },
            })
            .done(function (data) {
                if(true == data['auth']){
                    window.location.href = "{{  route('home') }}";
                }
            })
            .fail(function (data) {
                console.log(data);
                alert("Da co loi xay ra.");
            });
        });
    });
</script>
@endsection

