<!-- Window-popup Choose from my Photo -->

<div class="modal fade" id="choose-from-my-photo" tabindex="-1" role="dialog" aria-labelledby="choose-from-my-photo" aria-hidden="true">
        <div class="modal-dialog window-popup choose-from-my-photo" role="document">

            <div class="modal-content">
                <a href="./03-Newsfeed.html#" class="close icon-close" data-bs-dismiss="modal" aria-label="Close">
                    <svg class="olymp-close-icon">
                        <use xlink:href="#olymp-close-icon"></use>
                    </svg>
                </a>
                <div class="modal-header">
                    <h6 class="title">Choose from My Photos</h6>

                    <!-- Nav tabs -->
                    <ul class="nav nav-tabs" role="tablist">
                        <li class="nav-item">
                            <a class="nav-link active" data-bs-toggle="tab" href="./03-Newsfeed.html#home" role="tab" aria-expanded="true">
                                <svg class="olymp-photos-icon">
                                    <use xlink:href="#olymp-photos-icon"></use>
                                </svg>
                            </a>
                        </li>
                        <li class="nav-item">
                            <a class="nav-link" data-bs-toggle="tab" href="./03-Newsfeed.html#profile" role="tab" aria-expanded="false">
                                <svg class="olymp-albums-icon">
                                    <use xlink:href="#olymp-albums-icon"></use>
                                </svg>
                            </a>
                        </li>
                    </ul>
                </div>

                <div class="modal-body">
                    <!-- Tab panes -->
                    <div class="tab-content">
                        <div class="tab-pane fade active show" id="home" role="tabpanel" aria-expanded="true">

                            <div class="choose-photo-item">
                                <div class="radio">
                                    <label class="custom-radio">
                                        <img loading="lazy" src="{{asset('v1/ico/choose-photo1.webp')}}" alt="photo" width="247" height="166">
                                        <input type="radio" name="optionsRadios"><span class="circle"></span><span class="check"></span>
                                    </label>
                                </div>
                            </div>
                            <div class="choose-photo-item">
                                <div class="radio">
                                    <label class="custom-radio">
                                        <img loading="lazy" src="{{asset('v1/ico/choose-photo2.webp')}} " alt="photo" width="247" height="166">
                                        <input type="radio" name="optionsRadios"><span class="circle"></span><span class="check"></span>
                                    </label>
                                </div>
                            </div>
                            <div class="choose-photo-item">
                                <div class="radio">
                                    <label class="custom-radio">
                                        <img loading="lazy" src="{{asset('v1/ico/choose-photo3.webp')}}" alt="photo" width="247" height="166">
                                        <input type="radio" name="optionsRadios"><span class="circle"></span><span class="check"></span>
                                    </label>
                                </div>
                            </div>

                            <div class="choose-photo-item">
                                <div class="radio">
                                    <label class="custom-radio">
                                        <img loading="lazy" src="{{asset('v1/ico/choose-photo4.webp')}}" alt="photo" width="247" height="166">
                                        <input type="radio" name="optionsRadios"><span class="circle"></span><span class="check"></span>
                                    </label>
                                </div>
                            </div>
                            <div class="choose-photo-item">
                                <div class="radio">
                                    <label class="custom-radio">
                                        <img loading="lazy" src="{{asset('v1/ico/choose-photo5.webp')}} " alt="photo" width="247" height="166">
                                        <input type="radio" name="optionsRadios"><span class="circle"></span><span class="check"></span>
                                    </label>
                                </div>
                            </div>
                            <div class="choose-photo-item">
                                <div class="radio">
                                    <label class="custom-radio">
                                        <img loading="lazy" src="{{asset('v1/ico/choose-photo6.webp')}}" alt="photo" width="247" height="166">
                                        <input type="radio" name="optionsRadios"><span class="circle"></span><span class="check"></span>
                                    </label>
                                </div>
                            </div>

                            <div class="choose-photo-item">
                                <div class="radio">
                                    <label class="custom-radio">
                                        <img loading="lazy" src="{{asset('v1/ico/choose-photo7.webp')}} " alt="photo" width="247" height="166">
                                        <input type="radio" name="optionsRadios"><span class="circle"></span><span class="check"></span>
                                    </label>
                                </div>
                            </div>
                            <div class="choose-photo-item">
                                <div class="radio">
                                    <label class="custom-radio">
                                        <img loading="lazy" src="{{asset('v1/ico/choose-photo8.webp')}}" alt="photo" width="247" height="166">
                                        <input type="radio" name="optionsRadios"><span class="circle"></span><span class="check"></span>
                                    </label>
                                </div>
                            </div>
                            <div class="choose-photo-item">
                                <div class="radio">
                                    <label class="custom-radio">
                                        <img loading="lazy" src="{{asset('v1/ico/choose-photo9.webp')}}" alt="photo" width="247" height="166">
                                        <input type="radio" name="optionsRadios"><span class="circle"></span><span class="check"></span>
                                    </label>
                                </div>
                            </div>


                            <a href="./03-Newsfeed.html#" class="btn btn-secondary btn-lg btn--half-width">Cancel</a>
                            <a href="./03-Newsfeed.html#" class="btn btn-primary btn-lg btn--half-width">Confirm Photo</a>

                        </div>
                        <div class="tab-pane fade" id="profile" role="tabpanel" aria-expanded="false">

                            <div class="choose-photo-item">
                                <figure>
                                    <img loading="lazy" src="{{asset('v1/ico/choose-photo10.webp')}}" alt="photo" width="225" height="180">
                                    <figcaption>
                                        <a href="./03-Newsfeed.html#">South America Vacations</a>
                                        <span>Last Added: 2 hours ago</span>
                                    </figcaption>
                                </figure>
                            </div>
                            <div class="choose-photo-item">
                                <figure>
                                    <img loading="lazy" src="{{asset('v1/ico/choose-photo11.webp')}}" alt="photo" width="225" height="180">
                                    <figcaption>
                                        <a href="./03-Newsfeed.html#">Photoshoot Summer 2016</a>
                                        <span>Last Added: 5 weeks ago</span>
                                    </figcaption>
                                </figure>
                            </div>
                            <div class="choose-photo-item">
                                <figure>
                                    <img loading="lazy" src="{{asset('v1/ico/choose-photo12.webp')}} " alt="photo" width="225" height="180">
                                    <figcaption>
                                        <a href="./03-Newsfeed.html#">Amazing Street Food</a>
                                        <span>Last Added: 6 mins ago</span>
                                    </figcaption>
                                </figure>
                            </div>

                            <div class="choose-photo-item">
                                <figure>
                                    <img loading="lazy" src="{{asset('v1/ico/choose-photo13.webp')}}" alt="photo" width="224" height="179">
                                    <figcaption>
                                        <a href="./03-Newsfeed.html#">Graffity &amp; Street Art</a>
                                        <span>Last Added: 16 hours ago</span>
                                    </figcaption>
                                </figure>
                            </div>
                            <div class="choose-photo-item">
                                <figure>
                                    <img loading="lazy" src="{{asset('v1/ico/choose-photo14.webp')}} " alt="photo" width="225" height="180">
                                    <figcaption>
                                        <a href="./03-Newsfeed.html#">Amazing Landscapes</a>
                                        <span>Last Added: 13 mins ago</span>
                                    </figcaption>
                                </figure>
                            </div>
                            <div class="choose-photo-item">
                                <figure>
                                    <img loading="lazy" src="{{asset('v1/ico/choose-photo15.webp')}}" alt="photo" width="225" height="180">
                                    <figcaption>
                                        <a href="./03-Newsfeed.html#">The Majestic Canyon</a>
                                        <span>Last Added: 57 mins ago</span>
                                    </figcaption>
                                </figure>
                            </div>


                            <a href="./03-Newsfeed.html#" class="btn btn-secondary btn-lg btn--half-width">Cancel</a>
                            <a href="./03-Newsfeed.html#" class="btn btn-primary btn-lg disabled btn--half-width">Confirm Photo</a>
                        </div>
                    </div>
                </div>
            </div>

        </div>
    </div>

    <!-- ... end Window-popup Choose from my Photo -->