<?php

use Illuminate\Support\Facades\Schema;
use Illuminate\Database\Schema\Blueprint;
use Illuminate\Database\Migrations\Migration;

class CreateSessionTable extends Migration
{
    /**
     * Run the migrations.
     *
     * @return void
     */
    public function up()
    {
        Schema::create('session', function (Blueprint $table) {
            $table->id();
            $table->integer('session_id')->default(0);
            $table->integer('user_id')->default(0);
            $table->string('name')->default('none_value');
            $table->string('token')->default('none_value');
            $table->string('status')->default('none_value');
            $table->timestamp('expiration')->default(now());
            $table->timestamps();





        });
    }

    /**
     * Reverse the migrations.
     *
     * @return void
     */
    public function down()
    {
        Schema::dropIfExists('session');
    }


}



