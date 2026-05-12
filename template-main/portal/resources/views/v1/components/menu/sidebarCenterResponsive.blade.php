<header class="header header-responsive" id="site-header-responsive">

        <div class="header-content-wrapper">
            <ul class="nav nav-tabs mobile-notification-tabs" id="mobile-notification-tabs" role="tablist">
                <li class="nav-item" role="presentation">
                    <a class="nav-link" id="request-tab" data-bs-toggle="tab" href="./03-Newsfeed.html#request" role="tab" aria-controls="request" aria-selected="false">
                        <div class="control-icon has-items">
                            <svg class="olymp-happy-face-icon">
                                <use xlink:href="#olymp-happy-face-icon"></use>
                            </svg>
                            <div class="label-avatar bg-blue">6</div>
                        </div>
                    </a>
                </li>

                <li class="nav-item" role="presentation">
                    <a class="nav-link" id="chat-tab" data-bs-toggle="tab" href="./03-Newsfeed.html#chat" role="tab" aria-controls="chat" aria-selected="false">
                        <div class="control-icon has-items">
                            <svg class="olymp-chat---messages-icon">
                                <use xlink:href="#olymp-chat---messages-icon"></use>
                            </svg>
                            <div class="label-avatar bg-purple">2</div>
                        </div>
                    </a>
                </li>

                <li class="nav-item" role="presentation">
                    <a class="nav-link" id="notification-tab" data-bs-toggle="tab" href="./03-Newsfeed.html#notification" role="tab" aria-controls="notification" aria-selected="false">
                        <div class="control-icon has-items">
                            <svg class="olymp-thunder-icon">
                                <use xlink:href="#olymp-thunder-icon"></use>
                            </svg>
                            <div class="label-avatar bg-primary">8</div>
                        </div>
                    </a>
                </li>

                <li class="nav-item" role="presentation">
                    <a class="nav-link" id="search-tab" data-bs-toggle="tab" href="./03-Newsfeed.html#search" role="tab" aria-controls="search" aria-selected="false">
                        <svg class="olymp-magnifying-glass-icon">
                            <use xlink:href="#olymp-magnifying-glass-icon"></use>
                        </svg>
                        <svg class="olymp-close-icon">
                            <use xlink:href="#olymp-close-icon"></use>
                        </svg>
                    </a>
                </li>
            </ul>
        </div>

        <!-- Tab panes -->
        <div class="tab-content tab-content-responsive">

            <div class="tab-pane fade" id="request" role="tabpanel" aria-labelledby="request-tab">

                <div class="mCustomScrollbar ps ps--theme_default" data-mcs-theme="dark" data-ps-id="c261d31e-d3a7-09c0-970a-826408bd8602">
                    <div class="ui-block-title ui-block-title-small">
                        <h6 class="title">FRIEND REQUESTS</h6>
                        <a href="./03-Newsfeed.html#">Find Friends</a>
                        <a href="./03-Newsfeed.html#">Settings</a>
                    </div>
                    <ul class="notification-list friend-requests">
                        <li>
                            <div class="author-thumb">
                                <img loading="lazy" src="{{asset('v1/ico/avatar55-sm.webp')}} " alt="author" width="34" height="34">
                            </div>
                            <div class="notification-event">
                                <a href="./03-Newsfeed.html#" class="h6 notification-friend">Tamara Romanoff</a>
                                <span class="chat-message-item">Mutual Friend: Sarah Hetfield</span>
                            </div>
                            <span class="notification-icon">
                                <a href="./03-Newsfeed.html#" class="accept-request">
                                    <span class="icon-add without-text">
                                        <svg class="olymp-happy-face-icon">
                                            <use xlink:href="#olymp-happy-face-icon"></use>
                                        </svg>
                                    </span>
                                </a>

                                <a href="./03-Newsfeed.html#" class="accept-request request-del">
                                    <span class="icon-minus">
                                        <svg class="olymp-happy-face-icon">
                                            <use xlink:href="#olymp-happy-face-icon"></use>
                                        </svg>
                                    </span>
                                </a>

                            </span>

                            <div class="more">
                                <svg class="olymp-three-dots-icon">
                                    <use xlink:href="#olymp-three-dots-icon"></use>
                                </svg>
                            </div>
                        </li>
                        <li>
                            <div class="author-thumb">
                                <img loading="lazy" src="{{asset('v1/ico/avatar56-sm.webp')}}" alt="author" width="34" height="34">
                            </div>
                            <div class="notification-event">
                                <a href="./03-Newsfeed.html#" class="h6 notification-friend">Tony Stevens</a>
                                <span class="chat-message-item">4 Friends in Common</span>
                            </div>
                            <span class="notification-icon">
                                <a href="./03-Newsfeed.html#" class="accept-request">
                                    <span class="icon-add without-text">
                                        <svg class="olymp-happy-face-icon">
                                            <use xlink:href="#olymp-happy-face-icon"></use>
                                        </svg>
                                    </span>
                                </a>

                                <a href="./03-Newsfeed.html#" class="accept-request request-del">
                                    <span class="icon-minus">
                                        <svg class="olymp-happy-face-icon">
                                            <use xlink:href="#olymp-happy-face-icon"></use>
                                        </svg>
                                    </span>
                                </a>

                            </span>

                            <div class="more">
                                <svg class="olymp-three-dots-icon">
                                    <use xlink:href="#olymp-three-dots-icon"></use>
                                </svg>
                            </div>
                        </li>
                        <li class="accepted">
                            <div class="author-thumb">
                                <img loading="lazy" src="{{asset('v1/ico/avatar57-sm.webp')}}" alt="author" width="34" height="34">
                            </div>
                            <div class="notification-event">
                                You and
                                <a href="./03-Newsfeed.html#" class="h6 notification-friend">Mary Jane Stark</a> just became friends. Write on
                                <a href="./03-Newsfeed.html#" class="notification-link">her wall</a>.
                            </div>
                            <span class="notification-icon">
                                <svg class="olymp-happy-face-icon">
                                    <use xlink:href="#olymp-happy-face-icon"></use>
                                </svg>
                            </span>

                            <div class="more">
                                <svg class="olymp-three-dots-icon">
                                    <use xlink:href="#olymp-three-dots-icon"></use>
                                </svg>
                                <svg class="olymp-little-delete">
                                    <use xlink:href="#olymp-little-delete"></use>
                                </svg>
                            </div>
                        </li>
                        <li>
                            <div class="author-thumb">
                                <img loading="lazy" src="{{asset('v1/ico/avatar58-sm.webp')}}" alt="author" width="34" height="34">
                            </div>
                            <div class="notification-event">
                                <a href="./03-Newsfeed.html#" class="h6 notification-friend">Stagg Clothing</a>
                                <span class="chat-message-item">9 Friends in Common</span>
                            </div>
                            <span class="notification-icon">
                                <a href="./03-Newsfeed.html#" class="accept-request">
                                    <span class="icon-add without-text">
                                        <svg class="olymp-happy-face-icon">
                                            <use xlink:href="#olymp-happy-face-icon"></use>
                                        </svg>
                                    </span>
                                </a>

                                <a href="./03-Newsfeed.html#" class="accept-request request-del">
                                    <span class="icon-minus">
                                        <svg class="olymp-happy-face-icon">
                                            <use xlink:href="#olymp-happy-face-icon"></use>
                                        </svg>
                                    </span>
                                </a>

                            </span>

                            <div class="more">
                                <svg class="olymp-three-dots-icon">
                                    <use xlink:href="#olymp-three-dots-icon"></use>
                                </svg>
                            </div>
                        </li>
                    </ul>
                    <a href="./03-Newsfeed.html#" class="view-all bg-blue">Check all your Events</a>
                    <div class="ps__scrollbar-x-rail" style="left: 0px; bottom: 0px;">
                        <div class="ps__scrollbar-x" tabindex="0" style="left: 0px; width: 0px;"></div>
                    </div>
                    <div class="ps__scrollbar-y-rail" style="top: 0px; right: 0px;">
                        <div class="ps__scrollbar-y" tabindex="0" style="top: 0px; height: 0px;"></div>
                    </div>
                </div>

            </div>

            <div class="tab-pane fade" id="chat" role="tabpanel" aria-labelledby="chat-tab">

                <div class="mCustomScrollbar ps ps--theme_default" data-mcs-theme="dark" data-ps-id="d4715b5e-4383-158f-20f3-773ea3782dda">
                    <div class="ui-block-title ui-block-title-small">
                        <h6 class="title">Chat / Messages</h6>
                        <a href="./03-Newsfeed.html#">Mark all as read</a>
                        <a href="./03-Newsfeed.html#">Settings</a>
                    </div>

                    <ul class="notification-list chat-message">
                        <li class="message-unread">
                            <div class="author-thumb">
                                <img loading="lazy" src="{{asset('v1/ico/avatar59-sm.webp')}}" alt="author" width="34" height="34">
                            </div>
                            <div class="notification-event">
                                <a href="./03-Newsfeed.html#" class="h6 notification-friend">Diana Jameson</a>
                                <span class="chat-message-item">Hi James! It’s Diana, I just wanted to let you know that we have to reschedule...</span>
                                <span class="notification-date"><time class="entry-date updated" datetime="2004-07-24T18:18">4 hours ago</time></span>
                            </div>
                            <span class="notification-icon">
                                <svg class="olymp-chat---messages-icon">
                                    <use xlink:href="#olymp-chat---messages-icon"></use>
                                </svg>
                            </span>
                            <div class="more">
                                <svg class="olymp-three-dots-icon">
                                    <use xlink:href="#olymp-three-dots-icon"></use>
                                </svg>
                            </div>
                        </li>

                        <li>
                            <div class="author-thumb">
                                <img loading="lazy" src="{{asset('v1/ico/avatar60-sm.webp')}}" alt="author" width="34" height="34">
                            </div>
                            <div class="notification-event">
                                <a href="./03-Newsfeed.html#" class="h6 notification-friend">Jake Parker</a>
                                <span class="chat-message-item">Great, I’ll see you tomorrow!.</span>
                                <span class="notification-date"><time class="entry-date updated" datetime="2004-07-24T18:18">4 hours ago</time></span>
                            </div>
                            <span class="notification-icon">
                                <svg class="olymp-chat---messages-icon">
                                    <use xlink:href="#olymp-chat---messages-icon"></use>
                                </svg>
                            </span>

                            <div class="more">
                                <svg class="olymp-three-dots-icon">
                                    <use xlink:href="#olymp-three-dots-icon"></use>
                                </svg>
                            </div>
                        </li>
                        <li>
                            <div class="author-thumb">
                                <img loading="lazy" src="{{asset('v1/ico/avatar61-sm.webp')}}" alt="author" width="34" height="34">
                            </div>
                            <div class="notification-event">
                                <a href="./03-Newsfeed.html#" class="h6 notification-friend">Elaine Dreyfuss</a>
                                <span class="chat-message-item">We’ll have to check that at the office and see if the client is on board with...</span>
                                <span class="notification-date"><time class="entry-date updated" datetime="2004-07-24T18:18">Yesterday at 9:56pm</time></span>
                            </div>
                            <span class="notification-icon">
                                <svg class="olymp-chat---messages-icon">
                                    <use xlink:href="#olymp-chat---messages-icon"></use>
                                </svg>
                            </span>
                            <div class="more">
                                <svg class="olymp-three-dots-icon">
                                    <use xlink:href="#olymp-three-dots-icon"></use>
                                </svg>
                            </div>
                        </li>

                        <li class="chat-group">
                            <div class="author-thumb">
                                <img loading="lazy" src="{{asset('v1/ico/avatar11-sm.webp')}}" alt="author" width="16" height="16">
                                <img loading="lazy" src="{{asset('v1/ico/avatar12-sm.webp')}}" alt="author" width="16" height="16">
                                <img loading="lazy" src="{{asset('v1/ico/avatar13-sm.webp')}}" alt="author" width="16" height="16">
                                <img loading="lazy" src="{{asset('v1/ico/avatar10-sm.webp')}}" alt="author" width="36" height="36">
                            </div>
                            <div class="notification-event">
                                <a href="./03-Newsfeed.html#" class="h6 notification-friend">You, Faye, Ed &amp; Jet +3</a>
                                <span class="last-message-author">Ed:</span>
                                <span class="chat-message-item">Yeah! Seems fine by me!</span>
                                <span class="notification-date"><time class="entry-date updated" datetime="2004-07-24T18:18">March 16th at 10:23am</time></span>
                            </div>
                            <span class="notification-icon">
                                <svg class="olymp-chat---messages-icon">
                                    <use xlink:href="#olymp-chat---messages-icon"></use>
                                </svg>
                            </span>
                            <div class="more">
                                <svg class="olymp-three-dots-icon">
                                    <use xlink:href="#olymp-three-dots-icon"></use>
                                </svg>
                            </div>
                        </li>
                    </ul>

                    <a href="./03-Newsfeed.html#" class="view-all bg-purple">View All Messages</a>
                    <div class="ps__scrollbar-x-rail" style="left: 0px; bottom: 0px;">
                        <div class="ps__scrollbar-x" tabindex="0" style="left: 0px; width: 0px;"></div>
                    </div>
                    <div class="ps__scrollbar-y-rail" style="top: 0px; right: 0px;">
                        <div class="ps__scrollbar-y" tabindex="0" style="top: 0px; height: 0px;"></div>
                    </div>
                </div>

            </div>

            <div class="tab-pane fade" id="notification" role="tabpanel" aria-labelledby="notification-tab">

                <div class="mCustomScrollbar ps ps--theme_default" data-mcs-theme="dark" data-ps-id="c31f989e-3e2d-855d-d24a-02cf72f046d9">
                    <div class="ui-block-title ui-block-title-small">
                        <h6 class="title">Notifications</h6>
                        <a href="./03-Newsfeed.html#">Mark all as read</a>
                        <a href="./03-Newsfeed.html#">Settings</a>
                    </div>

                    <ul class="notification-list">
                        <li>
                            <div class="author-thumb">
                                <img loading="lazy" src="{{asset('v1/ico/avatar62-sm.webp')}} " width="34" height="34" alt="author">
                            </div>
                            <div class="notification-event">
                                <div><a href="./03-Newsfeed.html#" class="h6 notification-friend">Mathilda Brinker</a> commented on your new
                                    <a href="./03-Newsfeed.html#" class="notification-link">profile status</a>.
                                </div>
                                <span class="notification-date"><time class="entry-date updated" datetime="2004-07-24T18:18">4 hours ago</time></span>
                            </div>
                            <span class="notification-icon">
                                <svg class="olymp-comments-post-icon">
                                    <use xlink:href="#olymp-comments-post-icon"></use>
                                </svg>
                            </span>

                            <div class="more">
                                <svg class="olymp-three-dots-icon">
                                    <use xlink:href="#olymp-three-dots-icon"></use>
                                </svg>
                                <svg class="olymp-little-delete">
                                    <use xlink:href="#olymp-little-delete"></use>
                                </svg>
                            </div>
                        </li>

                        <li class="un-read">
                            <div class="author-thumb">
                                <img loading="lazy" src="{{asset('v1/ico/avatar63-sm.webp')}}" alt="author" width="34" height="34">
                            </div>
                            <div class="notification-event">
                                <div>You and
                                    <a href="./03-Newsfeed.html#" class="h6 notification-friend">Nicholas Grissom</a> just became friends. Write on
                                    <a href="./03-Newsfeed.html#" class="notification-link">his wall</a>.
                                </div>
                                <span class="notification-date"><time class="entry-date updated" datetime="2004-07-24T18:18">9 hours ago</time></span>
                            </div>
                            <span class="notification-icon">
                                <svg class="olymp-happy-face-icon">
                                    <use xlink:href="#olymp-happy-face-icon"></use>
                                </svg>
                            </span>

                            <div class="more">
                                <svg class="olymp-three-dots-icon">
                                    <use xlink:href="#olymp-three-dots-icon"></use>
                                </svg>
                                <svg class="olymp-little-delete">
                                    <use xlink:href="#olymp-little-delete"></use>
                                </svg>
                            </div>
                        </li>

                        <li class="with-comment-photo-wrap">
                            <div class="with-comment-photo">
                                <div class="author-thumb"><img loading="lazy" src="{{asset('v1/ico/avatar64-sm.webp')}}" width="34" height="34" alt="author"></div>
                                <div class="notification-event">
                                    <div><a href="./03-Newsfeed.html#" class="h6 notification-friend">Sarah Hetfield</a> commented on your
                                        <a href="./03-Newsfeed.html#" class="notification-link">photo</a>.
                                    </div>
                                    <span class="notification-date"><time class="entry-date updated" datetime="2004-07-24T18:18">Yesterday at 5:32am</time></span>
                                </div>
                                <span class="notification-icon"> <svg class="olymp-comments-post-icon">
                                        <use xlink:href="#olymp-comments-post-icon"></use>
                                    </svg> </span>
                            </div>
                            <div class="comment-photo"><img loading="lazy" src="{{asset('v1/ico/comment-photo1.webp')}} " alt="photo" width="40" height="40">
                                <span>“She looks incredible in that outfit! We should see each...”</span>
                            </div>
                            <div class="more">
                                <svg class="olymp-three-dots-icon">
                                    <use xlink:href="#olymp-three-dots-icon"></use>
                                </svg>
                                <svg class="olymp-little-delete">
                                    <use xlink:href="#olymp-little-delete"></use>
                                </svg>
                            </div>
                        </li>

                        <li>
                            <div class="author-thumb">
                                <img loading="lazy" src="{{asset('v1/ico/avatar65-sm.webp')}} " alt="author" width="34" height="34">
                            </div>
                            <div class="notification-event">
                                <div>
                                    <a href="./03-Newsfeed.html#" class="h6 notification-friend">Green Goo Rock</a> invited you to attend to his event Goo in
                                    <a href="./03-Newsfeed.html#" class="notification-link">Gotham Bar</a>.
                                </div>
                                <span class="notification-date"><time class="entry-date updated" datetime="2004-07-24T18:18">March 5th at 6:43pm</time></span>
                            </div>
                            <span class="notification-icon">
                                <svg class="olymp-happy-face-icon">
                                    <use xlink:href="#olymp-happy-face-icon"></use>
                                </svg>
                            </span>

                            <div class="more">
                                <svg class="olymp-three-dots-icon">
                                    <use xlink:href="#olymp-three-dots-icon"></use>
                                </svg>
                                <svg class="olymp-little-delete">
                                    <use xlink:href="#olymp-little-delete"></use>
                                </svg>
                            </div>
                        </li>

                        <li>
                            <div class="author-thumb">
                                <img loading="lazy" src="{{asset('v1/ico/avatar66-sm.webp')}}" alt="author" width="34" height="34">
                            </div>
                            <div class="notification-event">
                                <div><a href="./03-Newsfeed.html#" class="h6 notification-friend">James Summers</a> commented on your new
                                    <a href="./03-Newsfeed.html#" class="notification-link">profile status</a>.
                                </div>
                                <span class="notification-date"><time class="entry-date updated" datetime="2004-07-24T18:18">March 2nd at 8:29pm</time></span>
                            </div>
                            <span class="notification-icon">
                                <svg class="olymp-heart-icon">
                                    <use xlink:href="#olymp-heart-icon"></use>
                                </svg>
                            </span>

                            <div class="more">
                                <svg class="olymp-three-dots-icon">
                                    <use xlink:href="#olymp-three-dots-icon"></use>
                                </svg>
                                <svg class="olymp-little-delete">
                                    <use xlink:href="#olymp-little-delete"></use>
                                </svg>
                            </div>
                        </li>
                    </ul>

                    <a href="./03-Newsfeed.html#" class="view-all bg-primary">View All Notifications</a>
                    <div class="ps__scrollbar-x-rail" style="left: 0px; bottom: 0px;">
                        <div class="ps__scrollbar-x" tabindex="0" style="left: 0px; width: 0px;"></div>
                    </div>
                    <div class="ps__scrollbar-y-rail" style="top: 0px; right: 0px;">
                        <div class="ps__scrollbar-y" tabindex="0" style="top: 0px; height: 0px;"></div>
                    </div>
                </div>
            </div>
            <div class="tab-pane fade" id="search" role="tabpanel" aria-labelledby="search-tab">

                <form class="search-bar w-search notification-list friend-requests">
                    <div class="form-group with-button is-empty">
                        <input class="form-control js-user-search selectized" placeholder="Search here people or pages..." type="text" tabindex="-1" style="display: none;" value="">
                        <div class="selectize-control form-control js-user-search multi">
                            <div class="selectize-input items not-full has-options"><input type="text" autocomplete="off" tabindex="" placeholder="Search here people or pages..." style="width: 232.219px;"></div>
                            <div class="selectize-dropdown multi form-control js-user-search" style="display: none; width: 0px; top: 70px; left: 0px;">
                                <div class="selectize-dropdown-content"></div>
                            </div>
                        </div>
                        <span class="material-input"></span>
                    </div>
                </form>

            </div>

        </div>
        <!-- ... end  Tab panes -->
    </header>
