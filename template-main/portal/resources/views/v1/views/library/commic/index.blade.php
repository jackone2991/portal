@extends('v1.master.master-base')
@section('body-content')
    @php 
    $isAdmin = true ;
    @endphp
    <div class="container" >
        <div class="row ">
            <div class="col col-xl-12 col-12">
                <h2 class="presentation-margin">Thể Loại</h2>
            </div>
            @if($isAdmin)
                <div class="photo-album-item-wrap col-4-width">
                    <div class="photo-album-item create-album">
                         <a href="javascript:void(0)" data-bs-toggle="modal" data-bs-target="#create-photo-album" class="  full-block"></a>
                        <div class="content">
                            <a href="javascript:void(0)" class="btn btn-control bg-primary" data-bs-toggle="modal" data-bs-target="#create-photo-album">
                                <svg class="olymp-plus-icon"><use xlink:href="#olymp-plus-icon"></use></svg>
                            </a>
                            <a href="javascript:void(0)" class="title h5" data-bs-toggle="modal" data-bs-target="#create-photo-album">Create an Book</a>
                            <span class="sub-title">It only takes a few minutes!</span>
                        </div>
                    </div>
                </div>
            @endif
            <div class="photo-album-item-wrap col-4-width">	
                <div class="photo-album-item">
                    <div class="photo-item">
                        <img loading="lazy" src="{{ asset('v1/ico/photo-item2.webp') }}" alt="photo" width="332" height="284">
                        <div class="overlay overlay-dark"></div>
                        <a href="javascript:void(0)" class="more"><svg class="olymp-three-dots-icon"><use xlink:href="#olymp-three-dots-icon"></use></svg></a>
                        <a href="javascript:void(0)" class="post-add-icon">
                            <svg class="olymp-heart-icon"><use xlink:href="#olymp-heart-icon"></use></svg>
                            <span>324</span>
                        </a>
                        <a href="javascript:void(0)" data-bs-toggle="modal" data-bs-target="#open-photo-popup-v2" class="  full-block"></a>
                    </div>
                    <div class="content">
                        <a href="javascript:void(0)" class="title h5">South America Vacations</a>
                        <span class="sub-title">Last Added: 2 hours ago</span>
                
                        <div class="swiper-container swiper-swiper-unique-id-8 initialized swiper-container-horizontal" id="swiper-unique-id-8">
                            <div class="swiper-wrapper" style="width: 1020px; transform: translate3d(-255px, 0px, 0px); transition-duration: 0ms;"><div class="swiper-slide swiper-slide-duplicate swiper-slide-prev swiper-slide-duplicate-next" data-swiper-slide-index="1" style="width: 255px;">
                                    <div class="friend-count" data-swiper-parallax="-500" style="transform: translate3d(-500px, 0px, 0px); transition-duration: 0ms;">
                                        <a href="javascript:void(0)" class="friend-count-item">
                                            <div class="h6">24</div>
                                            <div class="title">Photos</div>
                                        </a>
                                        <a href="javascript:void(0)" class="friend-count-item">
                                            <div class="h6">86</div>
                                            <div class="title">Comments</div>
                                        </a>
                                        <a href="javascript:void(0)" class="friend-count-item">
                                            <div class="h6">16</div>
                                            <div class="title">Share</div>
                                        </a>
                                    </div>
                                </div>
                                <div class="swiper-slide swiper-slide-active" data-swiper-slide-index="0" style="width: 255px;">
                                    <ul class="friends-harmonic">
                                        <li>
                                            <a href="javascript:void(0)">
                                                <img loading="lazy" src="{{ asset('v1/ico/friend-harmonic5.webp') }}" alt="friend" width="28" height="28">
                                            </a>
                                        </li>
                                        <li>
                                            <a href="javascript:void(0)">
                                                <img loading="lazy" src="{{ asset('v1/ico/friend-harmonic10.webp') }}" alt="friend" width="28" height="28">
                                            </a>
                                        </li>
                                        <li>
                                            <a href="javascript:void(0)">
                                                <img loading="lazy" src="{{ asset('v1/ico/friend-harmonic7.webp') }}" alt="friend" width="28" height="28">
                                            </a>
                                        </li>
                                        <li>
                                            <a href="javascript:void(0)">
                                                <img loading="lazy" src="{{ asset('v1/ico/friend-harmonic8.webp') }}" alt="friend" width="28" height="28">
                                            </a>
                                        </li>
                                        <li>
                                            <a href="javascript:void(0)">
                                                <img loading="lazy" src="{{ asset('v1/ico/friend-harmonic2.webp') }}" alt="friend" width="28" height="28">
                                            </a>
                                        </li>
                                    </ul>
                                </div>
                                <div class="swiper-slide swiper-slide-next swiper-slide-duplicate-prev" data-swiper-slide-index="1" style="width: 255px;">
                                    <div class="friend-count" data-swiper-parallax="-500" style="transform: translate3d(500px, 0px, 0px); transition-duration: 0ms;">
                                        <a href="javascript:void(0)" class="friend-count-item">
                                            <div class="h6">24</div>
                                            <div class="title">Photos</div>
                                        </a>
                                        <a href="javascript:void(0)" class="friend-count-item">
                                            <div class="h6">86</div>
                                            <div class="title">Comments</div>
                                        </a>
                                        <a href="javascript:void(0)" class="friend-count-item">
                                            <div class="h6">16</div>
                                            <div class="title">Share</div>
                                        </a>
                                    </div>
                                </div>
                            <div class="swiper-slide swiper-slide-duplicate swiper-slide-duplicate-active" data-swiper-slide-index="0" style="width: 255px;">
                                    <ul class="friends-harmonic">
                                        <li>
                                            <a href="javascript:void(0)">
                                                <img loading="lazy" src="{{ asset('v1/ico/friend-harmonic5.webp') }}" alt="friend" width="28" height="28">
                                            </a>
                                        </li>
                                        <li>
                                            <a href="javascript:void(0)">
                                                <img loading="lazy" src="{{ asset('v1/ico/friend-harmonic10.webp') }}" alt="friend" width="28" height="28">
                                            </a>
                                        </li>
                                        <li>
                                            <a href="javascript:void(0)">
                                                <img loading="lazy" src="{{ asset('v1/ico/friend-harmonic7.webp') }}" alt="friend" width="28" height="28">
                                            </a>
                                        </li>
                                        <li>
                                            <a href="javascript:void(0)">
                                                <img loading="lazy" src="{{ asset('v1/ico/friend-harmonic8.webp') }}" alt="friend" width="28" height="28">
                                            </a>
                                        </li>
                                        <li>
                                            <a href="javascript:void(0)">
                                                <img loading="lazy" src="{{ asset('v1/ico/friend-harmonic2.webp') }}" alt="friend" width="28" height="28">
                                            </a>
                                        </li>
                                    </ul>
                                </div></div>
                            <!-- If we need pagination -->
                            <div class="swiper-pagination pagination-swiper-unique-id-8 swiper-pagination-clickable swiper-pagination-bullets"><span class="swiper-pagination-bullet swiper-pagination-bullet-active"></span><span class="swiper-pagination-bullet"></span></div>
                        </div>
                    </div>
                </div>
            </div>



        @include('v1.components.popup.addBook')
        </div>
    </div>
@endsection