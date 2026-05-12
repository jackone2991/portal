@extends('v1.master.master-base')
@section('body-content')
<div class="container">
    <div class="row">

        <!-- Main Content -->

        <main class="col col-xl-6 order-xl-2 col-lg-12 order-lg-1 col-md-12 col-sm-12 col-12">

            <div id="newsfeed-items-grid">


















                <div class="ui-block">

                    <article class="hentry post video">

                        <div class="post__author author vcard inline-items">
                            <img loading="lazy" src="{{ asset('v1/ico/friend-harmonic5.webp') }}" alt="author" width="42" height="42">

                            <div class="author-date">
                                <a class="h6 post__author-name fn" href="./03-Newsfeed.html#">The_Stranger</a> posted
                                <div class="post__date">
                                    <time class="published" datetime="2004-07-24T18:18">
                                        March 4 at 2:05pm
                                    </time>
                                </div>
                            </div>

                            @php
                            $isPoster = false ;
                            @endphp
                            @if ($isPoster)
                                <div class="more"><svg class="olymp-three-dots-icon">
                                        <use xlink:href="#olymp-three-dots-icon"></use>
                                    </svg>
                                    <ul class="more-dropdown">
                                        <li>
                                            <a href="./03-Newsfeed.html#">Edit Post</a>
                                        </li>
                                        <li>
                                            <a href="./03-Newsfeed.html#">Delete Post</a>
                                        </li>
                                        <li>
                                            <a href="./03-Newsfeed.html#">Turn Off Notifications</a>
                                        </li>
                                        <li>
                                            <a href="./03-Newsfeed.html#">Select as Featured</a>
                                        </li>
                                    </ul>
                                </div>
                            @endif

                        </div>

                        <img src="https://static.cdnno.com/storage/topbox/a45a7d96a1d1322c11b2899b64145b1f.jpg" alt="" />

                        <div class="control-block-button post-control-button">

                            <a href="./03-Newsfeed.html#" class="btn btn-control">
                                <svg class="olymp-like-post-icon">
                                    <use xlink:href="#olymp-like-post-icon"></use>
                                </svg>
                            </a>

                            <a href="./03-Newsfeed.html#" class="btn btn-control">
                                <svg class="olymp-comments-post-icon">
                                    <use xlink:href="#olymp-comments-post-icon"></use>
                                </svg>
                            </a>

                            <a href="./03-Newsfeed.html#" class="btn btn-control">
                                <svg class="olymp-share-icon">
                                    <use xlink:href="#olymp-share-icon"></use>
                                </svg>
                            </a>

                        </div>

                    </article>
                </div>
























































                <div class="ui-block">

                    <article class="hentry post video">

                        <div class="post__author author vcard inline-items">
                            <img loading="lazy" src="{{ asset('v1/ico/friend-harmonic5.webp') }}" alt="author" width="42" height="42">

                            <div class="author-date">
                                <a class="h6 post__author-name fn" href="./03-Newsfeed.html#">The_Stranger</a> posted
                                <div class="post__date">
                                    <time class="published" datetime="2004-07-24T18:18">
                                        March 4 at 2:05pm
                                    </time>
                                </div>
                            </div>

                            @php
                            $isPoster = false ;
                            @endphp
                            @if ($isPoster)
                                <div class="more"><svg class="olymp-three-dots-icon">
                                        <use xlink:href="#olymp-three-dots-icon"></use>
                                    </svg>
                                    <ul class="more-dropdown">
                                        <li>
                                            <a href="./03-Newsfeed.html#">Edit Post</a>
                                        </li>
                                        <li>
                                            <a href="./03-Newsfeed.html#">Delete Post</a>
                                        </li>
                                        <li>
                                            <a href="./03-Newsfeed.html#">Turn Off Notifications</a>
                                        </li>
                                        <li>
                                            <a href="./03-Newsfeed.html#">Select as Featured</a>
                                        </li>
                                    </ul>
                                </div>
                            @endif

                        </div>

                        <p>Hey <a href="./03-Newsfeed.html#">Cindi</a>, you should really check out this new song by Iron Maid. The next time they come to the city we should totally go!</p>




                        <div class="control-block-button post-control-button">

                            <a href="./03-Newsfeed.html#" class="btn btn-control">
                                <svg class="olymp-like-post-icon">
                                    <use xlink:href="#olymp-like-post-icon"></use>
                                </svg>
                            </a>

                            <a href="./03-Newsfeed.html#" class="btn btn-control">
                                <svg class="olymp-comments-post-icon">
                                    <use xlink:href="#olymp-comments-post-icon"></use>
                                </svg>
                            </a>

                            <a href="./03-Newsfeed.html#" class="btn btn-control">
                                <svg class="olymp-share-icon">
                                    <use xlink:href="#olymp-share-icon"></use>
                                </svg>
                            </a>

                        </div>

                    </article>
                </div>




























            </div>

            <a id="load-more-button" class="btn btn-control btn-more">
                <svg class="olymp-three-dots-icon">
                    <use xlink:href="#olymp-three-dots-icon"></use>
                </svg>
            </a>

        </main>

        <!-- ... end Main Content -->


        <!-- Left Sidebar -->

        <aside class="col col-xl-3 order-xl-1 col-lg-6 order-lg-2 col-md-6 col-sm-6 col-12">
            <div class="ui-block">

                <!-- W-Weather -->


                <!-- W-Weather -->
            </div>

            <div class="ui-block">

                <!-- W-Calendar -->



                <!-- ... end W-Calendar -->
            </div>

            <div class="ui-block">
                <div class="ui-block-title">
                    <h6 class="title">Pages You May Like</h6>
                    <a href="./03-Newsfeed.html#" class="more"><svg class="olymp-three-dots-icon">
                            <use xlink:href="#olymp-three-dots-icon"></use>
                        </svg></a>
                </div>

                <!-- W-Friend-Pages-Added -->

                <ul class="widget w-friend-pages-added notification-list friend-requests">
                    <li class="inline-items">
                        <div class="author-thumb">
                            <img loading="lazy" src="./ico/avatar41-sm.webp" alt="author" width="36" height="36">
                        </div>
                        <div class="notification-event">
                            <a href="./03-Newsfeed.html#" class="h6 notification-friend">The Marina Bar</a>
                            <span class="chat-message-item">Restaurant / Bar</span>
                        </div>
                        <span class="notification-icon" data-bs-toggle="tooltip" data-bs-placement="top" data-bs-original-title="ADD TO YOUR FAVS">
                            <a href="./03-Newsfeed.html#">
                                <svg class="olymp-star-icon">
                                    <use xlink:href="#olymp-star-icon"></use>
                                </svg>
                            </a>
                        </span>

                    </li>

                    <li class="inline-items">
                        <div class="author-thumb">
                            <img loading="lazy" src="./ico/avatar42-sm.webp" alt="author" width="36" height="36">
                        </div>
                        <div class="notification-event">
                            <a href="./03-Newsfeed.html#" class="h6 notification-friend">Tapronus Rock</a>
                            <span class="chat-message-item">Rock Band</span>
                        </div>
                        <span class="notification-icon" data-bs-toggle="tooltip" data-bs-placement="top" data-bs-original-title="ADD TO YOUR FAVS">
                            <a href="./03-Newsfeed.html#">
                                <svg class="olymp-star-icon">
                                    <use xlink:href="#olymp-star-icon"></use>
                                </svg>
                            </a>
                        </span>

                    </li>

                    <li class="inline-items">
                        <div class="author-thumb">
                            <img loading="lazy" src="./ico/avatar43-sm.webp" alt="author" width="36" height="36">
                        </div>
                        <div class="notification-event">
                            <a href="./03-Newsfeed.html#" class="h6 notification-friend">Pixel Digital Design</a>
                            <span class="chat-message-item">Company</span>
                        </div>
                        <span class="notification-icon" data-bs-toggle="tooltip" data-bs-placement="top" data-bs-original-title="ADD TO YOUR FAVS">
                            <a href="./03-Newsfeed.html#">
                                <svg class="olymp-star-icon">
                                    <use xlink:href="#olymp-star-icon"></use>
                                </svg>
                            </a>
                        </span>
                    </li>

                    <li class="inline-items">
                        <div class="author-thumb">
                            <img loading="lazy" src="./ico/avatar44-sm.webp" alt="author" width="36" height="36">
                        </div>
                        <div class="notification-event">
                            <a href="./03-Newsfeed.html#" class="h6 notification-friend">Thompson’s Custom Clothing Boutique</a>
                            <span class="chat-message-item">Clothing Store</span>
                        </div>
                        <span class="notification-icon" data-bs-toggle="tooltip" data-bs-placement="top" data-bs-original-title="ADD TO YOUR FAVS">
                            <a href="./03-Newsfeed.html#">
                                <svg class="olymp-star-icon">
                                    <use xlink:href="#olymp-star-icon"></use>
                                </svg>
                            </a>
                        </span>

                    </li>

                    <li class="inline-items">
                        <div class="author-thumb">
                            <img loading="lazy" src="./ico/avatar45-sm.webp" alt="author" width="36" height="36">
                        </div>
                        <div class="notification-event">
                            <a href="./03-Newsfeed.html#" class="h6 notification-friend">Crimson Agency</a>
                            <span class="chat-message-item">Company</span>
                        </div>
                        <span class="notification-icon" data-bs-toggle="tooltip" data-bs-placement="top" data-bs-original-title="ADD TO YOUR FAVS">
                            <a href="./03-Newsfeed.html#">
                                <svg class="olymp-star-icon">
                                    <use xlink:href="#olymp-star-icon"></use>
                                </svg>
                            </a>
                        </span>
                    </li>

                    <li class="inline-items">
                        <div class="author-thumb">
                            <img loading="lazy" src="./ico/avatar46-sm.webp" alt="author" width="38" height="38">
                        </div>
                        <div class="notification-event">
                            <a href="./03-Newsfeed.html#" class="h6 notification-friend">Mannequin Angel</a>
                            <span class="chat-message-item">Clothing Store</span>
                        </div>
                        <span class="notification-icon" data-bs-toggle="tooltip" data-bs-placement="top" data-bs-original-title="ADD TO YOUR FAVS">
                            <a href="./03-Newsfeed.html#">
                                <svg class="olymp-star-icon">
                                    <use xlink:href="#olymp-star-icon"></use>
                                </svg>
                            </a>
                        </span>
                    </li>
                </ul>

                <!-- .. end W-Friend-Pages-Added -->
            </div>
        </aside>

        <!-- ... end Left Sidebar -->


        <!-- Right Sidebar -->

        <aside class="col col-xl-3 order-xl-3 col-lg-6 order-lg-3 col-md-6 col-sm-6 col-12">

            <div class="ui-block">

                <!-- W-Birthsday-Alert -->



                <!-- ... end W-Birthsday-Alert -->
            </div>

            <div class="ui-block">
                <div class="ui-block-title">
                    <h6 class="title">Friend Suggestions</h6>
                    <a href="./03-Newsfeed.html#" class="more"><svg class="olymp-three-dots-icon">
                            <use xlink:href="#olymp-three-dots-icon"></use>
                        </svg></a>
                </div>



                <!-- W-Action -->

                <ul class="widget w-friend-pages-added notification-list friend-requests">
                    <li class="inline-items">
                        <div class="author-thumb">
                            <img loading="lazy" src="./ico/avatar38-sm.webp" alt="author" width="36" height="36">
                        </div>
                        <div class="notification-event">
                            <a href="./03-Newsfeed.html#" class="h6 notification-friend">Francine Smith</a>
                            <span class="chat-message-item">8 Friends in Common</span>
                        </div>
                        <span class="notification-icon">
                            <a href="./03-Newsfeed.html#" class="accept-request">
                                <span class="icon-add without-text">
                                    <svg class="olymp-happy-face-icon">
                                        <use xlink:href="#olymp-happy-face-icon"></use>
                                    </svg>
                                </span>
                            </a>
                        </span>
                    </li>

                    <li class="inline-items">
                        <div class="author-thumb">
                            <img loading="lazy" src="./ico/avatar39-sm.webp" alt="author" width="36" height="36">
                        </div>
                        <div class="notification-event">
                            <a href="./03-Newsfeed.html#" class="h6 notification-friend">Hugh Wilson</a>
                            <span class="chat-message-item">6 Friends in Common</span>
                        </div>
                        <span class="notification-icon">
                            <a href="./03-Newsfeed.html#" class="accept-request">
                                <span class="icon-add without-text">
                                    <svg class="olymp-happy-face-icon">
                                        <use xlink:href="#olymp-happy-face-icon"></use>
                                    </svg>
                                </span>
                            </a>
                        </span>
                    </li>

                    <li class="inline-items">
                        <div class="author-thumb">
                            <img loading="lazy" src="./ico/avatar40-sm.webp" alt="author" width="36" height="36">
                        </div>
                        <div class="notification-event">
                            <a href="./03-Newsfeed.html#" class="h6 notification-friend">Karen Masters</a>
                            <span class="chat-message-item">6 Friends in Common</span>
                        </div>
                        <span class="notification-icon">
                            <a href="./03-Newsfeed.html#" class="accept-request">
                                <span class="icon-add without-text">
                                    <svg class="olymp-happy-face-icon">
                                        <use xlink:href="#olymp-happy-face-icon"></use>
                                    </svg>
                                </span>
                            </a>
                        </span>
                    </li>

                </ul>

                <!-- ... end W-Action -->
            </div>

            <div class="ui-block">

                <div class="ui-block-title">
                    <h6 class="title">Activity Feed</h6>
                    <a href="./03-Newsfeed.html#" class="more"><svg class="olymp-three-dots-icon">
                            <use xlink:href="#olymp-three-dots-icon"></use>
                        </svg></a>
                </div>


                <!-- W-Activity-Feed -->

                <ul class="widget w-activity-feed notification-list">
                    <li>
                        <div class="author-thumb">
                            <img loading="lazy" src="./ico/avatar49-sm.webp" alt="author" width="28" height="28">
                        </div>
                        <div class="notification-event">
                            <a href="./03-Newsfeed.html#" class="h6 notification-friend">Marina Polson</a> commented on Jason Mark’s <a href="./03-Newsfeed.html#" class="notification-link">photo.</a>.
                            <span class="notification-date"><time class="entry-date updated" datetime="2004-07-24T18:18">2 mins ago</time></span>
                        </div>
                    </li>

                    <li>
                        <div class="author-thumb">
                            <img loading="lazy" src="./ico/avatar9-sm.webp" alt="author" width="36" height="36">
                        </div>
                        <div class="notification-event">
                            <a href="./03-Newsfeed.html#" class="h6 notification-friend">Jake Parker </a> liked Nicholas Grissom’s <a href="./03-Newsfeed.html#" class="notification-link">status update.</a>.
                            <span class="notification-date"><time class="entry-date updated" datetime="2004-07-24T18:18">5 mins ago</time></span>
                        </div>
                    </li>

                    <li>
                        <div class="author-thumb">
                            <img loading="lazy" src="./ico/avatar50-sm.webp" alt="author" width="28" height="28">
                        </div>
                        <div class="notification-event">
                            <a href="./03-Newsfeed.html#" class="h6 notification-friend">Mary Jane Stark </a> added 20 new photos to her <a href="./03-Newsfeed.html#" class="notification-link">gallery album.</a>.
                            <span class="notification-date"><time class="entry-date updated" datetime="2004-07-24T18:18">12 mins ago</time></span>
                        </div>
                    </li>

                    <li>
                        <div class="author-thumb">
                            <img loading="lazy" src="./ico/avatar51-sm.webp" alt="author" width="28" height="28">
                        </div>
                        <div class="notification-event">
                            <a href="./03-Newsfeed.html#" class="h6 notification-friend">Nicholas Grissom </a> updated his profile <a href="./03-Newsfeed.html#" class="notification-link">photo</a>.
                            <span class="notification-date"><time class="entry-date updated" datetime="2004-07-24T18:18">1 hour ago</time></span>
                        </div>
                    </li>
                    <li>
                        <div class="author-thumb">
                            <img loading="lazy" src="./ico/avatar48-sm.webp" alt="author" width="28" height="28">
                        </div>
                        <div class="notification-event">
                            <a href="./03-Newsfeed.html#" class="h6 notification-friend">Marina Valentine </a> commented on Chris Greyson’s <a href="./03-Newsfeed.html#" class="notification-link">status update</a>.
                            <span class="notification-date"><time class="entry-date updated" datetime="2004-07-24T18:18">1 hour ago</time></span>
                        </div>
                    </li>

                    <li>
                        <div class="author-thumb">
                            <img loading="lazy" src="./ico/avatar52-sm.webp" alt="author" width="28" height="28">
                        </div>
                        <div class="notification-event">
                            <a href="./03-Newsfeed.html#" class="h6 notification-friend">Green Goo Rock </a> posted a <a href="./03-Newsfeed.html#" class="notification-link">status update</a>.
                            <span class="notification-date"><time class="entry-date updated" datetime="2004-07-24T18:18">1 hour ago</time></span>
                        </div>
                    </li>
                    <li>
                        <div class="author-thumb">
                            <img loading="lazy" src="./ico/avatar10-sm.webp" alt="author" width="36" height="36">
                        </div>
                        <div class="notification-event">
                            <a href="./03-Newsfeed.html#" class="h6 notification-friend">Elaine Dreyfuss </a> liked your <a href="./03-Newsfeed.html#" class="notification-link">blog post</a>.
                            <span class="notification-date"><time class="entry-date updated" datetime="2004-07-24T18:18">2 hours ago</time></span>
                        </div>
                    </li>
                    <li>
                        <div class="author-thumb">
                            <img loading="lazy" src="./ico/avatar10-sm.webp" alt="author" width="36" height="36">
                        </div>
                        <div class="notification-event">
                            <a href="./03-Newsfeed.html#" class="h6 notification-friend">Elaine Dreyfuss </a> commented on your <a href="./03-Newsfeed.html#" class="notification-link">blog post</a>.
                            <span class="notification-date"><time class="entry-date updated" datetime="2004-07-24T18:18">2 hours ago</time></span>
                        </div>
                    </li>
                    <li>
                        <div class="author-thumb">
                            <img loading="lazy" src="./ico/avatar53-sm.webp" alt="author" width="28" height="28">
                        </div>
                        <div class="notification-event">
                            <a href="./03-Newsfeed.html#" class="h6 notification-friend">Bruce Peterson </a> changed his <a href="./03-Newsfeed.html#" class="notification-link">profile picture</a>.
                            <span class="notification-date"><time class="entry-date updated" datetime="2004-07-24T18:18">15 hours ago</time></span>
                        </div>
                    </li>
                </ul>
                <!-- .. end W-Activity-Feed -->
            </div>
        </aside>
        <!-- ... end Right Sidebar -->
    </div>
</div>


<script type="text/javascript">

var loading = false;//to prevent duplicate
function loadNewContent(){
    // $.ajax({
    //     type:'GET',
    //     url: url_to_new_content
    //     success:function(data){
    //         if(data != ""){
    //             loading = false;
    //             $("#div-to-update").html(data);
    //         }
    //     }
    // });
    alert("Loading content");
}
//scroll DIV's Bottom
$('#load-more-button').on('scroll', function() {
    if($(this).scrollTop() + $(this).innerHeight() >= $(this)[0].scrollHeight) {
        if(!loading){
            loading = true;
            loadNewContent();//call function to load content when scroll reachs DIV bottom
        }
    }
})

//scroll to PAGE's bottom
var winTop = $(window).scrollTop();
var docHeight = $(document).height();
var winHeight = $(window).height();
if  ((winTop / (docHeight - winHeight)) > 0.95) {
    if(!loading){
        loading = true;
        loadNewContent();//call function to load content when scroll reachs PAGE bottom
    }
}
</script>
@endsection
